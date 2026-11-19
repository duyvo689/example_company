package handler

import (
	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) GetCompany(c *gin.Context) {
	result, err := h.service.GetCompany(c.Request.Context(), httpx.Claims(c).CompanyID)
	respond(c, result, err)
}

func (h *Handler) UpdateCompany(c *gin.Context) {
	var req domain.UpdateCompanyRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateCompany(c.Request.Context(), httpx.Claims(c).CompanyID, req)
	respond(c, result, err)
}

func (h *Handler) ListLocations(c *gin.Context) {
	page, size := httpx.Pagination(c)
	result, err := h.service.ListLocations(c.Request.Context(), httpx.Claims(c), page, size, c.Query("search"), c.Query("status"))
	respond(c, result, err)
}

func (h *Handler) GetLocation(c *gin.Context) {
	id, ok := idParam(c, "location_id")
	if !ok {
		return
	}
	result, err := h.service.GetLocation(c.Request.Context(), httpx.Claims(c), id)
	respond(c, result, err)
}

func (h *Handler) CreateLocation(c *gin.Context) {
	var req domain.LocationRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateLocation(c.Request.Context(), httpx.Claims(c).CompanyID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateLocation(c *gin.Context) {
	id, ok := idParam(c, "location_id")
	if !ok {
		return
	}
	var req domain.LocationUpdateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateLocation(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	respond(c, result, err)
}

func (h *Handler) DeleteLocation(c *gin.Context) {
	id, ok := idParam(c, "location_id")
	if !ok {
		return
	}
	if err := h.service.DeleteLocation(c.Request.Context(), httpx.Claims(c).CompanyID, id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) ListRoles(c *gin.Context) {
	page, size := httpx.Pagination(c)
	result, err := h.service.ListRoles(c.Request.Context(), page, size)
	respond(c, result, err)
}

func (h *Handler) CreateRole(c *gin.Context) {
	var req domain.RoleRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateRole(c.Request.Context(), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	id, ok := idParam(c, "role_id")
	if !ok {
		return
	}
	var req domain.RoleRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateRole(c.Request.Context(), id, req)
	respond(c, result, err)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	id, ok := idParam(c, "role_id")
	if !ok {
		return
	}
	if err := h.service.DeleteRole(c.Request.Context(), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) ListMemberships(c *gin.Context) {
	page, size := httpx.Pagination(c)
	result, err := h.service.ListMemberships(c.Request.Context(), httpx.Claims(c).CompanyID, page, size, c.Query("search"), c.Query("status"))
	respond(c, result, err)
}

func (h *Handler) GetMembership(c *gin.Context) {
	id, ok := idParam(c, "membership_id")
	if !ok {
		return
	}
	result, err := h.service.GetMembership(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}

func (h *Handler) CreateMembership(c *gin.Context) {
	var req domain.MembershipRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateMembership(c.Request.Context(), httpx.Claims(c).CompanyID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateMembership(c *gin.Context) {
	id, ok := idParam(c, "membership_id")
	if !ok {
		return
	}
	var req domain.MembershipUpdateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateMembership(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	respond(c, result, err)
}

func (h *Handler) DeleteMembership(c *gin.Context) {
	id, ok := idParam(c, "membership_id")
	if !ok {
		return
	}
	claims := httpx.Claims(c)
	if err := h.service.DeleteMembership(c.Request.Context(), claims.CompanyID, id, claims.MembershipID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) ListAssignments(c *gin.Context) {
	id, ok := idParam(c, "membership_id")
	if !ok {
		return
	}
	result, err := h.service.ListAssignments(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}

func (h *Handler) AddAssignment(c *gin.Context) {
	id, ok := idParam(c, "membership_id")
	if !ok {
		return
	}
	var req domain.AssignmentRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.AddAssignment(c.Request.Context(), httpx.Claims(c).CompanyID, id, req.LocationID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) DeleteAssignment(c *gin.Context) {
	membershipID, ok := idParam(c, "membership_id")
	if !ok {
		return
	}
	locationID, ok := idParam(c, "location_id")
	if !ok {
		return
	}
	if err := h.service.DeleteAssignment(c.Request.Context(), httpx.Claims(c).CompanyID, membershipID, locationID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func respond(c *gin.Context, result any, err error) {
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}
