package handler

import (
	"example-company/internal/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, size := httpx.Pagination(c)
	result, err := h.service.ListAuditLogs(c.Request.Context(), httpx.Claims(c).CompanyID, page, size, c.Query("action"), c.Query("membership_id"), c.Query("from"), c.Query("to"))
	respond(c, result, err)
}

func (h *Handler) GetAuditLog(c *gin.Context) {
	id, ok := idParam(c, "audit_log_id")
	if !ok {
		return
	}
	result, err := h.service.GetAuditLog(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}

func (h *Handler) ListIntegrationEvents(c *gin.Context) {
	page, size := httpx.Pagination(c)
	result, err := h.service.ListIntegrationEvents(c.Request.Context(), httpx.Claims(c).CompanyID, page, size, c.Query("status"))
	respond(c, result, err)
}

func (h *Handler) RetryIntegrationEvent(c *gin.Context) {
	id, ok := idParam(c, "event_id")
	if !ok {
		return
	}
	result, err := h.service.RetryIntegrationEvent(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}
