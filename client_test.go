package rpgo

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, 1071, "test-token")
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"code":200,"data":{"accessCode":"12345678"}}`))
	})

	if _, err := c.GetAccessCode(context.Background(), "12345678901234"); err != nil {
		t.Fatalf("GetAccessCode: %v", err)
	}

	const want = "Basic MTA3MTp0ZXN0LXRva2Vu" // base64("1071:test-token")
	if gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
}

func TestAPIVersionPath(t *testing.T) {
	var gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"code":200,"data":{"accessCode":"x"}}`))
	})

	if _, err := c.GetAccessCode(context.Background(), "123"); err != nil {
		t.Fatalf("GetAccessCode: %v", err)
	}
	if want := "/api/v2.6/access-code/123"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}

	c2 := NewClient(c.baseURL, 1071, "test-token", WithAPIVersion("v2.3"))
	if _, err := c2.GetAccessCode(context.Background(), "123"); err != nil {
		t.Fatalf("GetAccessCode: %v", err)
	}
	if want := "/api/v2.3/access-code/123"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":401,"message":"Некорректный идентификатор клиента и/или токен"}`))
	})

	_, err := c.GetAccessCode(context.Background(), "123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized || apiErr.Code != 401 {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestDoJSONEnvelopeErrorOnHTTP200(t *testing.T) {
	// сервер отдал транспортный HTTP 200, но код внутри конверта говорит об
	// ошибке — это тоже должно возвращаться как ошибка, а не как "успех" с
	// нулевым значением данных.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":400,"message":"Структура commonInfo невалидна"}`))
	})

	_, err := c.GetAccessCode(context.Background(), "123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Code != 400 || apiErr.Message == "" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestNewClientHasNoOverallTimeout(t *testing.T) {
	// client.Timeout ограничивает весь round trip, включая отправку тела —
	// это резало бы большие/медленные заливки писем. Защита от зависания
	// живёт на уровне транспорта (dial/TLS/response-header таймауты), а не
	// как плоский дедлайн на весь запрос.
	c := NewClient("http://localhost", 1, "token")
	if c.httpClient.Timeout != 0 {
		t.Errorf("httpClient.Timeout = %v, want 0 (no overall timeout)", c.httpClient.Timeout)
	}

	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", c.httpClient.Transport)
	}
	if transport.TLSHandshakeTimeout != defaultTLSHandshakeTimeout {
		t.Errorf("TLSHandshakeTimeout = %v, want %v", transport.TLSHandshakeTimeout, defaultTLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != defaultResponseHeaderTimeout {
		t.Errorf("ResponseHeaderTimeout = %v, want %v", transport.ResponseHeaderTimeout, defaultResponseHeaderTimeout)
	}
}

func TestNewClientSurvivesReplacedDefaultTransport(t *testing.T) {
	// некоторые mock/APM-библиотеки (gock и подобные) подменяют
	// http.DefaultTransport на свою реализацию http.RoundTripper —
	// defaultHTTPClient не должен на этом падать через .(*http.Transport).
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		panic("unused")
	})
	defer func() { http.DefaultTransport = orig }()

	var c *Client
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewClient panicked: %v", r)
			}
		}()
		c = NewClient("http://localhost", 1, "token")
	}()

	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport (fallback)", c.httpClient.Transport)
	}
	// фоллбэк-транспорт должен сохранять те же дефолты, что и обычный клон
	// http.DefaultTransport, а не быть голой структурой — иначе клиент
	// незаметно потерял бы поддержку прокси из окружения и вытеснение
	// простаивающих соединений.
	if transport.Proxy == nil {
		t.Error("fallback transport has no Proxy (lost ProxyFromEnvironment)")
	}
	if !transport.ForceAttemptHTTP2 {
		t.Error("fallback transport has ForceAttemptHTTP2 = false")
	}
	if transport.IdleConnTimeout == 0 {
		t.Error("fallback transport has IdleConnTimeout = 0 (idle connections never reaped)")
	}
}

func TestNewClientWithHTTPClientSkipsDefault(t *testing.T) {
	custom := &http.Client{}
	c := NewClient("http://localhost", 1, "token", WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Error("WithHTTPClient's client was not used as-is")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDoJSONEmptyBodyOnSuccessIsZeroValue(t *testing.T) {
	// isSuccessStatus принимает весь класс 2xx, включая 204 без тела —
	// такой ответ должен давать нулевое значение, а не ошибку декодирования
	// пустого JSON.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	code, err := c.GetAccessCode(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetAccessCode: %v", err)
	}
	if code != "" {
		t.Errorf("code = %q, want empty", code)
	}
}

func TestDoJSONCapsResponseBodySize(t *testing.T) {
	// без предела io.ReadAll буферизовал бы тело целиком в память — здесь
	// тело обрывается лимитом раньше валидного конца JSON, поэтому
	// декодирование должно провалиться понятной ошибкой, а не зависнуть или
	// съесть память процесса.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"data":{"accessCode":"`))
		w.Write(bytes.Repeat([]byte("x"), maxResponseBodyBytes+1024))
	})

	_, err := c.GetAccessCode(context.Background(), "123")
	if err == nil {
		t.Fatal("expected a decode error for a body exceeding the size cap, got nil")
	}
}

func TestDoJSONAccepts2xxOtherThan200(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // 202
		w.Write([]byte(`{"code":202,"data":{"accessCode":"12345678"}}`))
	})

	code, err := c.GetAccessCode(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetAccessCode: %v", err)
	}
	if code != "12345678" {
		t.Errorf("code = %q", code)
	}
}

func TestDoRawAccepts2xxOtherThan200(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // 202
		w.Write([]byte("zip bytes"))
	})

	rc, err := c.GetDeliveryNoticesContent(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetDeliveryNoticesContent: %v", err)
	}
	rc.Close()
}

func TestAPIErrorNonJSONBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`<html>502 Bad Gateway</html>`))
	})

	_, err := c.GetAccessCode(context.Background(), "123")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.HTTPStatus != http.StatusBadGateway {
		t.Errorf("HTTPStatus = %d, want 502", apiErr.HTTPStatus)
	}
	if apiErr.Message != "" {
		t.Errorf("Message = %q, want empty", apiErr.Message)
	}
}
