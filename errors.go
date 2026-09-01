package rpgo

import (
	"encoding/json"
	"fmt"
)

// APIError возвращается на любой ответ API ИС ЭПС со статусом не 200.
// Message и Code заполняются, если тело ответа декодируется в
// документированную обёртку {code, message}; иначе сырой ответ доступен в
// Body (ответы с ошибкой на бинарных эндпоинтах, например содержимого
// уведомлений или ф103, не гарантированно приходят в JSON).
type APIError struct {
	HTTPStatus int
	Code       int
	Message    string
	Body       []byte
}

func (e *APIError) Error() string {
	switch {
	case e.Message != "":
		return fmt.Sprintf("rpgo: %s (http %d, code %d)", e.Message, e.HTTPStatus, e.Code)
	case e.Code != 0:
		return fmt.Sprintf("rpgo: unexpected http status %d (code %d)", e.HTTPStatus, e.Code)
	default:
		return fmt.Sprintf("rpgo: unexpected http status %d", e.HTTPStatus)
	}
}

func parseAPIError(status int, body []byte) error {
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &env); err == nil && (env.Code != 0 || env.Message != "") {
		return &APIError{HTTPStatus: status, Code: env.Code, Message: env.Message, Body: body}
	}
	return &APIError{HTTPStatus: status, Body: body}
}
