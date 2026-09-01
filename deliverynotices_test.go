package rpgo

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestCreateDeliveryNotices(t *testing.T) {
	var gotBody string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Write([]byte(`{"code":200,"data":{"id":"26cf8801-6ec6-4f99-95d3-d82cbf6a49b6"}}`))
	})

	id, err := c.CreateDeliveryNotices(context.Background(), NoticeTypeXML, []string{"123456789", "987654321"})
	if err != nil {
		t.Fatalf("CreateDeliveryNotices: %v", err)
	}
	if id != "26cf8801-6ec6-4f99-95d3-d82cbf6a49b6" {
		t.Errorf("id = %q", id)
	}
	if want := `{"noticeType":"XML","ids":["123456789","987654321"]}`; gotBody != want {
		t.Errorf("body = %q, want %q", gotBody, want)
	}
}

func TestGetDeliveryNoticesStatus(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"data":{"status":"IN_PROGRESS"}}`))
	})

	status, err := c.GetDeliveryNoticesStatus(context.Background(), "26cf8801-6ec6-4f99-95d3-d82cbf6a49b6")
	if err != nil {
		t.Fatalf("GetDeliveryNoticesStatus: %v", err)
	}
	if status != NoticeStatusInProgress {
		t.Errorf("status = %q", status)
	}
}

func TestGetDeliveryNoticesContent(t *testing.T) {
	const wantZip = "PK\x03\x04 fake zip bytes"
	var gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(wantZip))
	})

	rc, err := c.GetDeliveryNoticesContent(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetDeliveryNoticesContent: %v", err)
	}
	defer rc.Close()

	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != wantZip {
		t.Errorf("body = %q", body)
	}
	if want := "/api/v2.6/delivery-notices-p61/abc/content"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestGetDeliveryNoticesContentJSONErrorOnHTTP200(t *testing.T) {
	// документированные ошибки бинарных эндпоинтов приходят с не-2xx
	// статусом, но если шлюз перед ИС ЭПС всё же отдаст JSON-конверт с
	// ошибкой на HTTP 200, это не должно молча вернуться вызывающему коду
	// как содержимое zip-архива.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":500,"message":"Внутренняя ошибка сервиса"}`))
	})

	_, err := c.GetDeliveryNoticesContent(context.Background(), "abc")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Message != "Внутренняя ошибка сервиса" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestGetDeliveryNoticesContentError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":404,"message":"Указанный запрос не найден"}`))
	})

	_, err := c.GetDeliveryNoticesContent(context.Background(), "missing")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.HTTPStatus != http.StatusNotFound {
		t.Errorf("HTTPStatus = %d", apiErr.HTTPStatus)
	}
}
