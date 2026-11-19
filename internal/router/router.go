package router

import (
	"net/http"

	"example-company/internal/handler"
	"example-company/internal/httpx"
	"example-company/internal/middleware"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(db *gorm.DB, h *handler.Handler, tokens *platformjwt.Manager, middlewares ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(middlewares...)

	r.GET("/health/live", func(c *gin.Context) { httpx.OK(c, gin.H{"status": "ok"}) })
	r.GET("/health/ready", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, httpx.Envelope{Error: &httpx.ErrorBody{Code: "not_ready", Message: "database is unavailable"}})
			return
		}
		httpx.OK(c, gin.H{"status": "ready"})
	})

	v1 := r.Group("/api/v1")
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", h.Logout)

	protected := v1.Group("")
	protected.Use(middleware.Authenticate(tokens), middleware.ValidateSession(db))
	protected.GET("/me", h.Me)
	protected.PATCH("/me", h.UpdateMe)
	protected.PATCH("/me/password", h.ChangePassword)
	protected.GET("/company", h.GetCompany)
	protected.PATCH("/company", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.UpdateCompany)

	protected.GET("/locations", h.ListLocations)
	protected.GET("/locations/:location_id", h.GetLocation)
	protected.POST("/locations", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.CreateLocation)
	protected.PATCH("/locations/:location_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.UpdateLocation)
	protected.DELETE("/locations/:location_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.DeleteLocation)

	protected.GET("/roles", h.ListRoles)
	protected.POST("/roles", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.CreateRole)
	protected.PUT("/roles/:role_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.UpdateRole)
	protected.DELETE("/roles/:role_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.DeleteRole)

	protected.GET("/memberships", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.ListMemberships)
	protected.GET("/memberships/:membership_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.GetMembership)
	protected.POST("/memberships", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.CreateMembership)
	protected.PATCH("/memberships/:membership_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.UpdateMembership)
	protected.DELETE("/memberships/:membership_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.DeleteMembership)
	protected.GET("/memberships/:membership_id/locations", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.ListAssignments)
	protected.POST("/memberships/:membership_id/locations", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.AddAssignment)
	protected.DELETE("/memberships/:membership_id/locations/:location_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.DeleteAssignment)

	protected.GET("/patients", h.ListPatients)
	protected.GET("/patients/:patient_id", h.GetPatient)
	protected.POST("/patients", h.CreatePatient)
	protected.PATCH("/patients/:patient_id", h.UpdatePatient)
	protected.DELETE("/patients/:patient_id", middleware.RequireRoles("ADMIN"), h.DeletePatient)
	protected.POST("/patients/:patient_id/locations", h.AddPatientLocation)
	protected.DELETE("/patients/:patient_id/locations/:location_id", h.DeletePatientLocation)

	protected.GET("/orders", h.ListOrders)
	protected.GET("/orders/:order_id", h.GetOrder)
	protected.POST("/orders", h.CreateOrder)
	protected.PATCH("/orders/:order_id", h.UpdateOrder)
	protected.DELETE("/orders/:order_id", middleware.RequireRoles("ADMIN"), h.DeleteOrder)

	protected.GET("/audit-logs", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.ListAuditLogs)
	protected.GET("/audit-logs/:audit_log_id", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.GetAuditLog)
	protected.GET("/integration-events", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.ListIntegrationEvents)
	protected.POST("/integration-events/:event_id/retry", middleware.RequireRoles("ADMIN"), middleware.RequireCompanyScope(), h.RetryIntegrationEvent)

	return r
}
