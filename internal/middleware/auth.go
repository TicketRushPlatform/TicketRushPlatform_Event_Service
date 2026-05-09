package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthConfig struct {
	JWTSecret    string
	JWTAlgorithm string
}

type AuthClaims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Type string `json:"type"`
	jwt.RegisteredClaims
}

func RequireAuth(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(rawHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "Bearer access token is required.",
			})
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(rawHeader, "Bearer "))
		claims := &AuthClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if cfg.JWTAlgorithm != "" && token.Method.Alg() != cfg.JWTAlgorithm {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid || claims.Type != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "Access token is invalid or expired.",
			})
			return
		}
		if _, err := uuid.Parse(claims.Sub); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "Token subject is invalid.",
			})
			return
		}

		c.Set("auth_sub", claims.Sub)
		c.Set("auth_role", strings.ToUpper(strings.TrimSpace(claims.Role)))
		c.Next()
	}
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	sub, exists := c.Get("auth_sub")
	if !exists {
		return uuid.Nil, false
	}
	idStr, ok := sub.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func GetRole(c *gin.Context) string {
	role, exists := c.Get("auth_role")
	if !exists {
		return ""
	}
	roleStr, _ := role.(string)
	return roleStr
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := strings.ToUpper(strings.TrimSpace(c.GetString("auth_role")))
		if role != "ADMIN" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    http.StatusForbidden,
				"message": "Admin role is required.",
			})
			return
		}
		c.Next()
	}
}

func RequireAnyRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[strings.ToUpper(strings.TrimSpace(role))] = struct{}{}
	}
	return func(c *gin.Context) {
		role := strings.ToUpper(strings.TrimSpace(c.GetString("auth_role")))
		if _, ok := allowed[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    http.StatusForbidden,
				"message": "You do not have permission for this action.",
			})
			return
		}
		c.Next()
	}
}
