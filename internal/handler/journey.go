package handler

import (
	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ListCareJourneys(c *gin.Context) {
	page, size := httpx.Pagination(c)
	patientID, ok := optionalUUIDQuery(c, "patient_id")
	if !ok {
		return
	}
	locationID, ok := optionalUUIDQuery(c, "location_id")
	if !ok {
		return
	}
	result, err := h.service.ListCareJourneys(c.Request.Context(), httpx.Claims(c), page, size, c.Query("status"), c.Query("program_code"), patientID, locationID)
	respond(c, result, err)
}

func (h *Handler) GetCareJourney(c *gin.Context) {
	id, ok := idParam(c, "journey_id")
	if !ok {
		return
	}
	result, err := h.service.GetCareJourney(c.Request.Context(), httpx.Claims(c), id)
	respond(c, result, err)
}

func (h *Handler) CreateCareJourney(c *gin.Context) {
	var req domain.CreateCareJourneyRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateCareJourney(c.Request.Context(), httpx.Claims(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateCareJourney(c *gin.Context) {
	id, ok := idParam(c, "journey_id")
	if !ok {
		return
	}
	var req domain.UpdateCareJourneyRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateCareJourney(c.Request.Context(), httpx.Claims(c), id, req)
	respond(c, result, err)
}

func (h *Handler) CreateJourneyStep(c *gin.Context) {
	journeyID, ok := idParam(c, "journey_id")
	if !ok {
		return
	}
	var req domain.JourneyStepRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateJourneyStep(c.Request.Context(), httpx.Claims(c), journeyID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateJourneyStep(c *gin.Context) {
	stepID, ok := idParam(c, "step_id")
	if !ok {
		return
	}
	var req domain.UpdateJourneyStepRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateJourneyStep(c.Request.Context(), httpx.Claims(c), stepID, req)
	respond(c, result, err)
}

func (h *Handler) RecordChurnEvent(c *gin.Context) {
	journeyID, ok := idParam(c, "journey_id")
	if !ok {
		return
	}
	var req domain.ChurnEventRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.RecordChurnEvent(c.Request.Context(), httpx.Claims(c), journeyID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) ListIntakeSubmissions(c *gin.Context) {
	page, size := httpx.Pagination(c)
	journeyID, ok := optionalUUIDQuery(c, "journey_id")
	if !ok {
		return
	}
	patientID, ok := optionalUUIDQuery(c, "patient_id")
	if !ok {
		return
	}
	result, err := h.service.ListIntakeSubmissions(c.Request.Context(), httpx.Claims(c), page, size, c.Query("intake_type"), c.Query("status"), journeyID, patientID)
	respond(c, result, err)
}

func (h *Handler) GetIntakeSubmission(c *gin.Context) {
	id, ok := idParam(c, "submission_id")
	if !ok {
		return
	}
	result, err := h.service.GetIntakeSubmission(c.Request.Context(), httpx.Claims(c), id)
	respond(c, result, err)
}

func (h *Handler) CreateIntakeSubmission(c *gin.Context) {
	var req domain.CreateIntakeSubmissionRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateIntakeSubmission(c.Request.Context(), httpx.Claims(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) AddIntakeFile(c *gin.Context) {
	submissionID, ok := idParam(c, "submission_id")
	if !ok {
		return
	}
	var req domain.CreateIntakeFileRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.AddIntakeFile(c.Request.Context(), httpx.Claims(c), submissionID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) IssueRenewalLink(c *gin.Context) {
	stepID, ok := idParam(c, "step_id")
	if !ok {
		return
	}
	var req domain.IssueRenewalLinkRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.IssueRenewalLink(c.Request.Context(), httpx.Claims(c), stepID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) ListRenewalLinks(c *gin.Context) {
	stepID, ok := idParam(c, "step_id")
	if !ok {
		return
	}
	result, err := h.service.ListRenewalLinks(c.Request.Context(), httpx.Claims(c), stepID)
	respond(c, result, err)
}

func (h *Handler) OpenRenewalLink(c *gin.Context) {
	var req domain.OpenRenewalLinkRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.OpenRenewalLink(c.Request.Context(), c.Param("token"), req.VerificationValue)
	respond(c, result, err)
}

func (h *Handler) SubmitRenewal(c *gin.Context) {
	var req domain.SubmitRenewalRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, created, err := h.service.SubmitRenewal(c.Request.Context(), c.Param("token"), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if created {
		httpx.Created(c, result)
		return
	}
	httpx.OK(c, result)
}
