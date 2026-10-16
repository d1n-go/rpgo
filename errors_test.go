package rpgo

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAPIErrorMessage(t *testing.T) {
	cases := []struct {
		err  *APIError
		want string
	}{
		{&APIError{HTTPStatus: 401, Code: 401, Message: "bad token"}, "rpgo: bad token (http 401, code 401)"},
		{&APIError{HTTPStatus: 200, Code: 400}, "rpgo: unexpected http status 200 (code 400)"},
		{&APIError{HTTPStatus: 502}, "rpgo: unexpected http status 502"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

// endpointCalls вызывает каждый публичный метод Client и возвращает только
// ошибку — так ветки обработки ошибок проверяются одинаково для всех
// эндпоинтов, а не только для того, на котором написан профильный тест.
func endpointCalls() map[string]func(c *Client) error {
	ctx := context.Background()
	return map[string]func(c *Client) error{
		"SendLetter": func(c *Client) error {
			_, err := c.SendLetter(ctx, CommonInfo{Type: MailTypeRegistered})
			return err
		},
		"GetLetterStatus": func(c *Client) error {
			_, err := c.GetLetterStatus(ctx, "req_1")
			return err
		},
		"CreateDeliveryNotices": func(c *Client) error {
			_, err := c.CreateDeliveryNotices(ctx, NoticeTypeXML, []string{"1"})
			return err
		},
		"GetDeliveryNoticesStatus": func(c *Client) error {
			_, err := c.GetDeliveryNoticesStatus(ctx, "id")
			return err
		},
		"GetDeliveryNoticesContent": func(c *Client) error {
			_, err := c.GetDeliveryNoticesContent(ctx, "id")
			return err
		},
		"BatchElectronicDeliveryNoticeStatus": func(c *Client) error {
			_, err := c.BatchElectronicDeliveryNoticeStatus(ctx, []string{"1"})
			return err
		},
		"GetElectronicDeliveryNotice": func(c *Client) error {
			_, _, err := c.GetElectronicDeliveryNotice(ctx, "1")
			return err
		},
		"GetAccessCode": func(c *Client) error {
			_, err := c.GetAccessCode(ctx, "1")
			return err
		},
		"GetAccessHistory": func(c *Client) error {
			_, err := c.GetAccessHistory(ctx, "1")
			return err
		},
		"GetF103Registry": func(c *Client) error {
			_, err := c.GetF103Registry(ctx, time.Date(2025, 9, 5, 0, 0, 0, 0, time.UTC))
			return err
		},
	}
}

func TestEndpointsReturnAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"code":500,"message":"Внутренняя ошибка сервиса"}`))
	})

	for name, call := range endpointCalls() {
		t.Run(name, func(t *testing.T) {
			var apiErr *APIError
			if err := call(c); !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiErr.HTTPStatus != http.StatusInternalServerError || apiErr.Code != 500 {
				t.Errorf("apiErr = %+v", apiErr)
			}
		})
	}
}

func TestEndpointsReturnTransportError(t *testing.T) {
	transportErr := errors.New("connection refused")
	c := NewClient("http://example.invalid", 1, "token", WithHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, transportErr
		}),
	}))

	for name, call := range endpointCalls() {
		t.Run(name, func(t *testing.T) {
			if err := call(c); !errors.Is(err, transportErr) {
				t.Errorf("err = %v, want wrapping %v", err, transportErr)
			}
		})
	}
}

func TestEndpointsRejectInvalidBaseURL(t *testing.T) {
	// управляющий символ в URL не даёт собрать запрос — ошибка должна
	// вернуться из newRequest, до какой-либо сетевой активности.
	c := NewClient("http://bad\x7fhost", 1, "token")

	for name, call := range endpointCalls() {
		t.Run(name, func(t *testing.T) {
			err := call(c)
			if err == nil || !strings.Contains(err.Error(), "rpgo: build request") {
				t.Errorf("err = %v, want build request error", err)
			}
		})
	}
}

func TestTruncatedResponseBodyIsReadError(t *testing.T) {
	// сервер объявил Content-Length больше, чем отдал, и закрыл соединение —
	// чтение тела должно вернуть ошибку, а не молча обработать обрубок.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte(`{"code":200,`))
	})

	for _, name := range []string{"GetAccessCode", "GetF103Registry"} {
		t.Run(name, func(t *testing.T) {
			err := endpointCalls()[name](c)
			if err == nil || !strings.Contains(err.Error(), "rpgo: read response body") {
				t.Errorf("err = %v, want read response body error", err)
			}
		})
	}
}
