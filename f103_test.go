package rpgo

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestGetF103Registry(t *testing.T) {
	const wantZip = "PK\x03\x04 fake f103 registry"
	var gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(wantZip))
	})

	date := time.Date(2025, 9, 5, 0, 0, 0, 0, time.UTC)
	rc, err := c.GetF103Registry(context.Background(), date)
	if err != nil {
		t.Fatalf("GetF103Registry: %v", err)
	}
	defer rc.Close()

	if want := "/api/v2.6/f103/20250905"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	body, _ := io.ReadAll(rc)
	if string(body) != wantZip {
		t.Errorf("body = %q", body)
	}
}

func TestGetF103RegistryUsesDateAsIsRegardlessOfZone(t *testing.T) {
	// date, уже выражающий календарную дату в зоне восточнее Москвы (здесь —
	// условный UTC+10, как во Владивостоке), не должен пересчитываться и
	// "съезжать" на соседний день: используются его собственные год/месяц/
	// день, а не заново вычисленные после перевода в московскую зону.
	vladivostok := time.FixedZone("VLAT", 10*60*60)

	var gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	})

	date := time.Date(2025, 9, 5, 1, 0, 0, 0, vladivostok) // 05.09 01:00 VLAT == 04.09 18:00 MSK
	rc, err := c.GetF103Registry(context.Background(), date)
	if err != nil {
		t.Fatalf("GetF103Registry: %v", err)
	}
	rc.Close()

	if want := "/api/v2.6/f103/20250905"; gotPath != want {
		t.Errorf("path = %q, want %q (date's own Y/M/D, not shifted to Moscow's calendar day)", gotPath, want)
	}
}
