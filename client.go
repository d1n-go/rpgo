package rpgo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// defaultAPIVersion — сегмент URL-пути с версией API, которую сейчас
// документирует Почта России (см. «Памятка по подключению клиентов к ИС
// ЭПС», раздел 6). Более старые версии (v1.0, v2.3) использовали другие
// пути для тех же операций; если по вашему договору нужна другая версия —
// зафиксируйте её через WithAPIVersion.
const defaultAPIVersion = "v2.6"

// defaultDialTimeout, defaultTLSHandshakeTimeout и
// defaultResponseHeaderTimeout ограничивают отдельные фазы соединения, а не
// запрос целиком. Так вызов без дедлайна в ctx не может зависнуть навсегда
// на мёртвом узле (исходная проблема http.DefaultClient — у него нет вовсе
// никаких таймаутов), но при этом не режется сама передача тела: письмо с
// вложениями по медленному ГОСТ/VipNet-каналу может передаваться сколь
// угодно долго, лишь бы сервер вовремя откликался. Плоский
// http.Client.Timeout так не умеет — он ограничивает весь round trip
// целиком, включая отправку тела.
const (
	defaultDialTimeout           = 10 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultResponseHeaderTimeout = 30 * time.Second
)

// maxResponseBodyBytes ограничивает, сколько doJSON/doRaw готовы прочитать в
// память с JSON-конвертов и тел ошибок. Такие тела всегда небольшие
// (структуры Response/APIError, а не сам бинарный контент — тот отдаётся
// вызывающему коду как поток, без буферизации целиком); без предела
// неисправный прокси/шлюз, вернувший огромное тело, мог бы исчерпать
// память процесса через один io.ReadAll.
const maxResponseBodyBytes = 10 << 20 // 10 MiB

func readLimitedBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBodyBytes))
}

func defaultHTTPClient() *http.Client {
	// http.DefaultTransport обычно *http.Transport, но некоторые
	// mock/APM-библиотеки (gock и подобные) подменяют его на свою
	// реализацию http.RoundTripper — падать в этом случае некорректно, раз
	// withHTTPClient и так позволяет полностью обойти этот дефолт.
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		// строим транспорт с теми же дефолтами, что использует сам net/http
		// для http.DefaultTransport, а не пустую структуру — иначе клиент
		// молча потерял бы поддержку прокси из окружения (HTTP_PROXY и
		// т.п.), апгрейд до HTTP/2 и вытеснение простаивающих соединений.
		transport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
	transport.DialContext = (&net.Dialer{Timeout: defaultDialTimeout, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	return &http.Client{Transport: transport}
}

// Client работает с REST API Почты России ИС ЭПС.
type Client struct {
	baseURL    string
	apiVersion string
	customerID int
	authToken  string
	httpClient *http.Client
}

// Option настраивает Client, созданный через NewClient.
type Option func(*Client)

// WithHTTPClient переопределяет http.Client, используемый для запросов.
// Пригодится, чтобы подставить клиент с собственным Transport (например,
// подключающимся к локальному ГОСТ-TLS/VipNet-туннелю) или с обычным
// плоским Timeout, если вам нужен именно он, а не поведение по умолчанию.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithAPIVersion переопределяет версию API в пути (по умолчанию "v2.6").
func WithAPIVersion(version string) Option {
	return func(c *Client) { c.apiVersion = version }
}

// NewClient создаёт Client для API ИС ЭПС, доступного по baseURL (например,
// "http://localhost:3080", если ГОСТ-TLS/VipNet-туннель уже терминирован
// локально). customerID и authToken — идентификатор отправителя и токен,
// выданные Почтой России; из них собирается заголовок Basic-авторизации,
// как описано в документации.
func NewClient(baseURL string, customerID int, authToken string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiVersion: defaultAPIVersion,
		customerID: customerID,
		authToken:  authToken,
	}
	for _, opt := range opts {
		opt(c)
	}
	// defaultHTTPClient клонирует транспорт и заводит свой dialer — не
	// стоит его строить, если WithHTTPClient всё равно сейчас же его
	// заменит.
	if c.httpClient == nil {
		c.httpClient = defaultHTTPClient()
	}
	return c
}

func (c *Client) authHeader() string {
	creds := strconv.Itoa(c.customerID) + ":" + c.authToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

// path строит "/api/<версия><suffix>" простой конкатенацией, без
// форматных глаголов — suffix должен быть уже полностью статической
// строкой (без пользовательских данных). Для путей с динамическим
// сегментом (ID, ШПИ и т.п.) используйте pathf.
func (c *Client) path(suffix string) string {
	return "/api/" + c.apiVersion + suffix
}

// pathf строит "/api/<версия><prefix>/<id><suffix>", экранируя id через
// url.PathEscape самостоятельно — так вызывающему коду не нужно (и
// невозможно забыть) звать url.PathEscape вручную, и в id не может
// случайно затесаться verb вроде "%s", ломающий фматирование.
func (c *Client) pathf(prefix, id, suffix string) string {
	return "/api/" + c.apiVersion + prefix + "/" + url.PathEscape(id) + suffix
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("rpgo: build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader())
	return req, nil
}

// isSuccessStatus сообщает, считается ли HTTP- или встроенный в конверт код
// успехом. API не документирует ничего, кроме 200, но резервировать весь
// класс 2xx (201/202/204 и т.п.) безопаснее, чем требовать буквально 200.
func isSuccessStatus(code int) bool {
	return code >= 200 && code < 300
}

// envelope — общая обёртка ответа {code, message, data}, документированная
// для каждого JSON-эндпоинта.
type envelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// doJSON выполняет req и декодирует поле "data" успешного ответа в T. При
// неуспешном HTTP-статусе возвращает *APIError. Документация утверждает,
// что поле code внутри тела "соответствует http-коду ответа" — на случай,
// если сервер всё же вернёт HTTP 2xx с кодом ошибки внутри конверта
// (например, code 400 при валидном транспортном ответе), это тоже
// трактуется как ошибка, а не как «успех с пустыми данными».
func doJSON[T any](c *Client, req *http.Request) (T, error) {
	var zero T

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("rpgo: %w", err)
	}
	defer resp.Body.Close()

	body, err := readLimitedBody(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("rpgo: read response body: %w", err)
	}

	if !isSuccessStatus(resp.StatusCode) {
		return zero, parseAPIError(resp.StatusCode, body)
	}

	// API документирует ответы только с телом, но раз isSuccessStatus
	// принимает весь класс 2xx, пустое тело на 204 (или на 200/202 без
	// содержимого) — это не ошибка декодирования, а просто отсутствие
	// данных.
	if len(bytes.TrimSpace(body)) == 0 {
		return zero, nil
	}

	var env envelope[T]
	if err := json.Unmarshal(body, &env); err != nil {
		return zero, fmt.Errorf("rpgo: decode response body: %w", err)
	}

	if env.Code != 0 && !isSuccessStatus(env.Code) {
		return zero, &APIError{HTTPStatus: resp.StatusCode, Code: env.Code, Message: env.Message, Body: body}
	}

	return env.Data, nil
}

// postJSON маршалит payload в JSON, отправляет его POST-запросом на path и
// декодирует "data" из ответа в TResp — это общая форма, которую разделяют
// все JSON-эндпоинты с телом запроса.
func postJSON[TResp any](c *Client, ctx context.Context, path string, payload any) (TResp, error) {
	var zero TResp

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return zero, fmt.Errorf("rpgo: marshal request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(reqBody))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")

	return doJSON[TResp](c, req)
}

// doRaw выполняет req и возвращает *http.Response успешного ответа как
// есть, чтобы вызывающий код мог при необходимости прочитать заголовки
// (например, Content-Type) поверх тела. Закрыть resp.Body — обязанность
// вызывающего кода. При неуспешном статусе возвращает *APIError.
//
// Документированные ошибки бинарных эндпоинтов (скачивание содержимого
// уведомлений, ф103 и т.п.) всегда приходят с не-2xx статусом, но на
// случай, если прокси/шлюз перед ИС ЭПС всё же отдаст ошибку с HTTP 2xx —
// ровно то, от чего явно защищается doJSON, — здесь дополнительно
// проверяется Content-Type: успешный ответ бинарного эндпоинта никогда не
// application/json, так что такой ответ разбирается как конверт с ошибкой,
// а не отдаётся вызывающему коду как бинарное содержимое.
func (c *Client) doRaw(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rpgo: %w", err)
	}

	if !isSuccessStatus(resp.StatusCode) {
		defer resp.Body.Close()
		body, _ := readLimitedBody(resp.Body)
		return nil, parseAPIError(resp.StatusCode, body)
	}

	if mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); mediaType == "application/json" {
		defer resp.Body.Close()
		body, err := readLimitedBody(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("rpgo: read response body: %w", err)
		}
		return nil, parseAPIError(resp.StatusCode, body)
	}

	return resp, nil
}
