package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewHandlerServesIndex 验证 console 根路径会返回前端入口页面。
func TestNewHandlerServesIndex(t *testing.T) {
	handler := NewHandler()
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if contentType := rr.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
}

// TestNewHandlerFallsBackToIndex 验证前端路由会回退到 SPA 入口而不是直接 404。
func TestNewHandlerFallsBackToIndex(t *testing.T) {
	handler := NewHandler()
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/providers/primary", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "ModelMux Control Plane") {
		t.Fatalf("body = %q, want console title", rr.Body.String())
	}
}
