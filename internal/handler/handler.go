package handler

import (
	"example-company/internal/httpx"
	"example-company/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *service.Service }

func New(service *service.Service) *Handler { return &Handler{service: service} }

func idParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := httpx.UUIDParam(c, name)
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid "+name))
		return uuid.Nil, false
	}
	return id, true
}

func optionalUUIDQuery(c *gin.Context, name string) (*uuid.UUID, bool) {
	value := c.Query(name)
	if value == "" {
		return nil, true
	}
	id, err := uuid.Parse(value)
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid "+name))
		return nil, false
	}
	return &id, true
}
