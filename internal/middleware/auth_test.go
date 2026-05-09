package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func signedHS256Token(t *testing.T, secret string, claims *AuthClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	userID := uuid.New()
	cfg := AuthConfig{JWTSecret: secret, JWTAlgorithm: "HS256"}

	validClaims := &AuthClaims{
		Sub:  userID.String(),
		Role: "event_owner",
		Type: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	tests := []struct {
		name       string
		header     string
		cfgAlg     string
		wantStatus int
	}{
		{name: "missing bearer", header: "", wantStatus: http.StatusUnauthorized},
		{name: "not bearer prefix", header: "Basic x", wantStatus: http.StatusUnauthorized},
		{name: "invalid jwt", header: "Bearer x.y.z", wantStatus: http.StatusUnauthorized},
		{
			name:       "wrong alg vs config",
			header:     "Bearer " + signedHS256Token(t, secret, validClaims),
			cfgAlg:     "RS256",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "wrong token type",
			header: "Bearer " + signedHS256Token(t, secret, &AuthClaims{
				Sub:  userID.String(),
				Role: "ADMIN",
				Type: "refresh",
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid subject uuid",
			header: "Bearer " + signedHS256Token(t, secret, &AuthClaims{
				Sub:  "bad",
				Role: "ADMIN",
				Type: "access",
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "success uppercases role",
			header:     "Bearer " + signedHS256Token(t, secret, validClaims),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCfg := cfg
			if tt.cfgAlg != "" {
				testCfg.JWTAlgorithm = tt.cfgAlg
			}
			r := gin.New()
			r.GET("/", RequireAuth(testCfg), func(c *gin.Context) {
				if tt.wantStatus != http.StatusOK {
					t.Fatal("handler should not run")
				}
				if c.GetString("auth_role") != "EVENT_OWNER" {
					t.Fatalf("role=%q want EVENT_OWNER", c.GetString("auth_role"))
				}
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/a", func(c *gin.Context) {
		c.Set("auth_role", "VIEWER")
		c.Next()
	}, RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/a", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}

	r2 := gin.New()
	r2.GET("/a", func(c *gin.Context) {
		c.Set("auth_role", "admin")
		c.Next()
	}, RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/a", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("status=%d want ok", w2.Code)
	}
}

func TestRequireAnyRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		c.Set("auth_role", "OTHER")
		c.Next()
	}, RequireAnyRole("EVENT_OWNER", "ADMIN"), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}

	r2 := gin.New()
	r2.GET("/x", func(c *gin.Context) {
		c.Set("auth_role", " event_owner ")
		c.Next()
	}, RequireAnyRole("EVENT_OWNER"), func(c *gin.Context) { c.Status(http.StatusOK) })
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("status=%d", w2.Code)
	}
}
