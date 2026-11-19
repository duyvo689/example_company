package handler

import (
	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ListPatients(c *gin.Context) {
	page, size := httpx.Pagination(c)
	locationID, ok := optionalUUIDQuery(c, "location_id")
	if !ok {
		return
	}
	result, err := h.service.ListPatients(c.Request.Context(), httpx.Claims(c), page, size, c.Query("search"), c.Query("status"), locationID)
	respond(c, result, err)
}

func (h *Handler) GetPatient(c *gin.Context) {
	id, ok := idParam(c, "patient_id")
	if !ok {
		return
	}
	result, err := h.service.GetPatient(c.Request.Context(), httpx.Claims(c), id)
	respond(c, result, err)
}

func (h *Handler) CreatePatient(c *gin.Context) {
	var req domain.PatientRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreatePatient(c.Request.Context(), httpx.Claims(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdatePatient(c *gin.Context) {
	id, ok := idParam(c, "patient_id")
	if !ok {
		return
	}
	var req domain.PatientUpdateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdatePatient(c.Request.Context(), httpx.Claims(c), id, req)
	respond(c, result, err)
}

func (h *Handler) DeletePatient(c *gin.Context) {
	id, ok := idParam(c, "patient_id")
	if !ok {
		return
	}
	if err := h.service.DeletePatient(c.Request.Context(), httpx.Claims(c), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) AddPatientLocation(c *gin.Context) {
	patientID, ok := idParam(c, "patient_id")
	if !ok {
		return
	}
	var req domain.PatientLocationRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.AddPatientLocation(c.Request.Context(), httpx.Claims(c), patientID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) DeletePatientLocation(c *gin.Context) {
	patientID, ok := idParam(c, "patient_id")
	if !ok {
		return
	}
	locationID, ok := idParam(c, "location_id")
	if !ok {
		return
	}
	if err := h.service.DeletePatientLocation(c.Request.Context(), httpx.Claims(c), patientID, locationID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
