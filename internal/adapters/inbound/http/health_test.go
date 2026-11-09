package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	api := nethttp.NewServeMux()
	handler := Mount(api, func(context.Context) error { return nil })

	req := httptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, nethttp.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want %q", body["status"], "ok")
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name       string
		ready      ReadyFunc
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ready",
			ready:      func(context.Context) error { return nil },
			wantStatus: nethttp.StatusOK,
			wantBody:   "ready",
		},
		{
			name:       "unavailable",
			ready:      func(context.Context) error { return errors.New("dependency down") },
			wantStatus: nethttp.StatusServiceUnavailable,
			wantBody:   "unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := nethttp.NewServeMux()
			handler := Mount(api, tt.ready)

			req := httptest.NewRequest(nethttp.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["status"] != tt.wantBody {
				t.Fatalf("status = %q, want %q", body["status"], tt.wantBody)
			}
		})
	}
}

func TestOperationalEndpointsOutsideAPIV1(t *testing.T) {
	api := nethttp.NewServeMux()
	handler := Mount(api, func(context.Context) error { return nil })

	for _, path := range []string{"/api/v1/healthz", "/api/v1/readyz"} {
		req := httptest.NewRequest(nethttp.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != nethttp.StatusNotFound {
			t.Fatalf("%s status = %d, want %d (must not be under /api/v1)", path, rec.Code, nethttp.StatusNotFound)
		}
	}
}
