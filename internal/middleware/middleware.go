package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" || len(id) > 64 {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
		}
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			_, matched := allowed[origin]
			if allowAll || matched {
				value := origin
				if allowAll {
					value = "*"
				}
				c.Header("Access-Control-Allow-Origin", value)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Max-Age", "86400")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched_route"
		}
		log.Info("http_request",
			"request_id", c.GetString("request_id"), "method", c.Request.Method,
			"path", path, "status", c.Writer.Status(),
			"latency_ms", time.Since(started).Milliseconds(), "ip", c.ClientIP(),
		)
	}
}

func Recover(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error("panic_recovered", "request_id", c.GetString("request_id"), "error", recovered)
		c.AbortWithStatusJSON(http.StatusInternalServerError, httpx.Envelope{Error: &httpx.ErrorBody{Code: "internal_error", Message: "an internal error occurred"}})
	})
}

func Authenticate(tokens *platformjwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, httpx.Envelope{Error: &httpx.ErrorBody{Code: "unauthorized", Message: "missing bearer token"}})
			return
		}
		claims, err := tokens.Parse(parts[1], platformjwt.AccessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, httpx.Envelope{Error: &httpx.ErrorBody{Code: "unauthorized", Message: "invalid or expired access token"}})
			return
		}
		c.Set(httpx.ClaimsKey, claims)
		c.Next()
	}
}

func ValidateSession(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := httpx.Claims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, httpx.Envelope{Error: &httpx.ErrorBody{Code: "unauthorized", Message: "authentication required"}})
			return
		}
		var session struct {
			MembershipStatus string
			UserStatus       string
			CompanyStatus    string
			RoleCode         string
			RoleScope        string
		}
		err := db.WithContext(c.Request.Context()).Table("memberships m").
			Select("m.status AS membership_status, u.status AS user_status, c.status AS company_status, r.code AS role_code, r.scope AS role_scope").
			Joins("JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL").
			Joins("JOIN companies c ON c.id = m.company_id AND c.deleted_at IS NULL").
			Joins("JOIN roles r ON r.id = m.role_id AND r.deleted_at IS NULL").
			Where("m.id = ? AND m.user_id = ? AND m.company_id = ? AND m.deleted_at IS NULL", claims.MembershipID, claims.UserID, claims.CompanyID).
			Take(&session).Error
		if err != nil || session.MembershipStatus != domain.StatusActive || session.UserStatus != domain.StatusActive || session.CompanyStatus != domain.StatusActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, httpx.Envelope{Error: &httpx.ErrorBody{Code: "session_inactive", Message: "account or membership is inactive"}})
			return
		}
		claims.RoleCode = session.RoleCode
		claims.Scope = session.RoleScope
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[strings.ToUpper(role)] = struct{}{}
	}
	return func(c *gin.Context) {
		claims := httpx.Claims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, httpx.Envelope{Error: &httpx.ErrorBody{Code: "unauthorized", Message: "authentication required"}})
			return
		}
		if _, ok := allowed[strings.ToUpper(claims.RoleCode)]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, httpx.Envelope{Error: &httpx.ErrorBody{Code: "forbidden", Message: "insufficient role"}})
			return
		}
		c.Next()
	}
}

func RequireCompanyScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := httpx.Claims(c)
		if claims == nil || claims.Scope != domain.ScopeCompany {
			c.AbortWithStatusJSON(http.StatusForbidden, httpx.Envelope{Error: &httpx.ErrorBody{Code: "forbidden", Message: "company scope is required"}})
			return
		}
		c.Next()
	}
}

func Audit(db *gorm.DB, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			return
		}
		metadata, _ := json.Marshal(map[string]any{"query": c.Request.URL.RawQuery})
		record := domain.AuditLog{
			RequestID: c.GetString("request_id"), Method: c.Request.Method, Path: c.FullPath(),
			Action: action(c.Request.Method), StatusCode: c.Writer.Status(), IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(), Metadata: datatypes.JSON(metadata), CreatedAt: time.Now(),
		}
		if claims := httpx.Claims(c); claims != nil {
			record.CompanyID = &claims.CompanyID
			record.MembershipID = &claims.MembershipID
		}
		if err := db.WithContext(c.Request.Context()).Create(&record).Error; err != nil {
			log.Error("write_audit_log", "request_id", record.RequestID, "error", err)
		}
	}
}

func action(method string) string {
	switch method {
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return strings.ToLower(method)
	}
}
