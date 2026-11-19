package handler

import (
	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req domain.RefreshRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Handler) Logout(c *gin.Context) {
	var req domain.RefreshRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) Me(c *gin.Context) {
	result, err := h.service.Me(c.Request.Context(), httpx.Claims(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Handler) UpdateMe(c *gin.Context) {
	var req domain.UpdateUserRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateMe(c.Request.Context(), httpx.Claims(c), req.Name)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req domain.ChangePasswordRequest
	if !httpx.Bind(c, &req) {
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), httpx.Claims(c), req.CurrentPassword, req.NewPassword); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
