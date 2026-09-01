package rpgo

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestBatchElectronicDeliveryNoticeStatus(t *testing.T) {
	var gotBody string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Write([]byte(`{"code":200,"data":{"statuses":[{"mailId":"12345678901","status":"OK"}]}}`))
	})

	statuses, err := c.BatchElectronicDeliveryNoticeStatus(context.Background(), []string{"12345678901"})
	if err != nil {
		t.Fatalf("BatchElectronicDeliveryNoticeStatus: %v", err)
	}
	if len(statuses) != 1 || statuses[0].MailID != "12345678901" || statuses[0].Status != ElectronicNoticeOK {
		t.Errorf("statuses = %+v", statuses)
	}
	if want := `{"mailIds":["12345678901"]}`; gotBody != want {
		t.Errorf("body = %q, want %q", gotBody, want)
	}
}

func TestGetElectronicDeliveryNotice(t *testing.T) {
	const wantJPEG = "\xff\xd8\xff fake jpeg bytes"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte(wantJPEG))
	})

	rc, contentType, err := c.GetElectronicDeliveryNotice(context.Background(), "12345678901")
	if err != nil {
		t.Fatalf("GetElectronicDeliveryNotice: %v", err)
	}
	defer rc.Close()

	if contentType != "image/jpeg" {
		t.Errorf("contentType = %q", contentType)
	}
	body, _ := io.ReadAll(rc)
	if string(body) != wantJPEG {
		t.Errorf("body = %q", body)
	}
}
