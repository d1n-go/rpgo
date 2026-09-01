package rpgo

import (
	"context"
	"net/http"
	"testing"
)

func TestGetAccessHistory(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// dateTime в документации назван String, но в примере — голое
		// число; Timestamp должен принимать оба варианта.
		w.Write([]byte(`{"code":200,"data":{"accessHistory":[{"dateTime":1535015817000,"fullName":"Иванов Иван Иванович"}]}}`))
	})

	history, err := c.GetAccessHistory(context.Background(), "12345678901234")
	if err != nil {
		t.Fatalf("GetAccessHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("len(history) = %d, want 1", len(history))
	}
	if history[0].FullName != "Иванов Иван Иванович" {
		t.Errorf("FullName = %q", history[0].FullName)
	}
	if history[0].DateTime.Time().Unix() != 1535015817 {
		t.Errorf("DateTime = %v", history[0].DateTime.Time())
	}
}

func TestGetAccessHistoryQuotedTimestamp(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"data":{"accessHistory":[{"dateTime":"1535015817000","fullName":"x"}]}}`))
	})

	history, err := c.GetAccessHistory(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetAccessHistory: %v", err)
	}
	if history[0].DateTime.Time().Unix() != 1535015817 {
		t.Errorf("DateTime = %v", history[0].DateTime.Time())
	}
}
