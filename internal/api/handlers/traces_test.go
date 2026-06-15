// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/go-chi/chi/v5"
)

func TestListTraceTags_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/traces", listTraceTags(stubReader{
		listTraceTags: func(_ context.Context, _ []string, _, _ string, _, _ time.Time, _ time.Time, _ int32) ([]api.TraceTagSummary, error) {
			return []api.TraceTagSummary{{TraceTag: "trace-001"}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/traces", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetTrace_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/traces/{tag}", getTrace(stubReader{
		getTraceByTag: func(_ context.Context, tag string) (*api.TraceDetail, error) {
			return &api.TraceDetail{TraceTag: tag}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/traces/trace-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetTrace_NotFound(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/traces/{tag}", getTrace(stubReader{
		getTraceByTag: func(_ context.Context, _ string) (*api.TraceDetail, error) {
			return nil, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/traces/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
