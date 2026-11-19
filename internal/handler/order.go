package handler

import (
	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ListOrders(c *gin.Context) {
	page, size := httpx.Pagination(c)
	locationID, ok := optionalUUIDQuery(c, "location_id")
	if !ok {
		return
	}
	patientID, ok := optionalUUIDQuery(c, "patient_id")
	if !ok {
		return
	}
	journeyStepID, ok := optionalUUIDQuery(c, "journey_step_id")
	if !ok {
		return
	}
	result, err := h.service.ListOrders(c.Request.Context(), httpx.Claims(c), page, size, c.Query("status"), locationID, patientID, journeyStepID)
	respond(c, result, err)
}

func (h *Handler) GetOrder(c *gin.Context) {
	id, ok := idParam(c, "order_id")
	if !ok {
		return
	}
	result, err := h.service.GetOrder(c.Request.Context(), httpx.Claims(c), id)
	respond(c, result, err)
}

func (h *Handler) CreateOrder(c *gin.Context) {
	var req domain.OrderRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateOrder(c.Request.Context(), httpx.Claims(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateOrder(c *gin.Context) {
	id, ok := idParam(c, "order_id")
	if !ok {
		return
	}
	var req domain.OrderUpdateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateOrder(c.Request.Context(), httpx.Claims(c), id, req)
	respond(c, result, err)
}

func (h *Handler) DeleteOrder(c *gin.Context) {
	id, ok := idParam(c, "order_id")
	if !ok {
		return
	}
	if err := h.service.DeleteOrder(c.Request.Context(), httpx.Claims(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
