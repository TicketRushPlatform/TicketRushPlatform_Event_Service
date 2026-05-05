package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		requestID  string
		wantStatus int
	}{
		{"normal request", http.MethodGet, "/x", "", http.StatusOK},
		{"options request", http.MethodOptions, "/x", "", http.StatusNoContent},
		{"request id passthrough", http.MethodGet, "/x", "abc", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(CORS(), RequestID(), RequestLogger(zap.NewNop()))
			r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.requestID != "" {
				req.Header.Set("X-Request-ID", tt.requestID)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d", w.Code, tt.wantStatus)
			}

			if tt.method != http.MethodOptions && w.Header().Get("X-Request-ID") == "" {
				t.Fatalf("expected X-Request-ID header")
			}
		})
	}
}
