package handler

import (
	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ListTreatmentPrograms(c *gin.Context) {
	page, size := httpx.Pagination(c)
	result, err := h.service.ListTreatmentPrograms(c.Request.Context(), httpx.Claims(c).CompanyID, page, size, c.Query("status"), c.Query("category"))
	respond(c, result, err)
}

func (h *Handler) GetTreatmentProgram(c *gin.Context) {
	id, ok := idParam(c, "program_id")
	if !ok {
		return
	}
	result, err := h.service.GetTreatmentProgram(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}

func (h *Handler) CreateTreatmentProgram(c *gin.Context) {
	var req domain.TreatmentProgramRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateTreatmentProgram(c.Request.Context(), httpx.Claims(c).CompanyID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateTreatmentProgram(c *gin.Context) {
	id, ok := idParam(c, "program_id")
	if !ok {
		return
	}
	var req domain.TreatmentProgramRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateTreatmentProgram(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	respond(c, result, err)
}

func (h *Handler) CreateProgramVersion(c *gin.Context) {
	id, ok := idParam(c, "program_id")
	if !ok {
		return
	}
	var req domain.ProgramVersionRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateProgramVersion(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) PublishProgramVersion(c *gin.Context) {
	id, ok := idParam(c, "version_id")
	if !ok {
		return
	}
	result, err := h.service.PublishProgramVersion(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}

func (h *Handler) CreateStepTemplate(c *gin.Context) {
	id, ok := idParam(c, "version_id")
	if !ok {
		return
	}
	var req domain.StepTemplateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateStepTemplate(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpdateStepTemplate(c *gin.Context) {
	id, ok := idParam(c, "step_template_id")
	if !ok {
		return
	}
	var req domain.StepTemplateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpdateStepTemplate(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	respond(c, result, err)
}

func (h *Handler) AddStepDependency(c *gin.Context) {
	id, ok := idParam(c, "step_template_id")
	if !ok {
		return
	}
	var req domain.StepDependencyRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.AddStepDependency(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) AssignFormToStep(c *gin.Context) {
	id, ok := idParam(c, "step_template_id")
	if !ok {
		return
	}
	var req domain.StepFormAssignmentRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.AssignFormToStep(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) UpsertProgramLocation(c *gin.Context) {
	id, ok := idParam(c, "program_id")
	if !ok {
		return
	}
	var req domain.ProgramLocationRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.UpsertProgramLocation(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	respond(c, result, err)
}

func (h *Handler) InstantiateTreatmentProgram(c *gin.Context) {
	id, ok := idParam(c, "program_id")
	if !ok {
		return
	}
	var req domain.InstantiateProgramRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.InstantiateTreatmentProgram(c.Request.Context(), httpx.Claims(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) ListWorkflows(c *gin.Context) {
	result, err := h.service.ListWorkflows(c.Request.Context(), httpx.Claims(c).CompanyID)
	respond(c, result, err)
}

func (h *Handler) CreateWorkflow(c *gin.Context) {
	var req domain.WorkflowTemplateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateWorkflow(c.Request.Context(), httpx.Claims(c).CompanyID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) PublishWorkflow(c *gin.Context) {
	id, ok := idParam(c, "workflow_id")
	if !ok {
		return
	}
	result, err := h.service.PublishWorkflow(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}

func (h *Handler) ListFormTemplates(c *gin.Context) {
	result, err := h.service.ListFormTemplates(c.Request.Context(), httpx.Claims(c).CompanyID)
	respond(c, result, err)
}

func (h *Handler) CreateFormTemplate(c *gin.Context) {
	var req domain.FormTemplateRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateFormTemplate(c.Request.Context(), httpx.Claims(c).CompanyID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) CreateFormVersion(c *gin.Context) {
	id, ok := idParam(c, "form_id")
	if !ok {
		return
	}
	var req domain.FormVersionRequest
	if !httpx.Bind(c, &req) {
		return
	}
	result, err := h.service.CreateFormVersion(c.Request.Context(), httpx.Claims(c).CompanyID, id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) PublishFormVersion(c *gin.Context) {
	id, ok := idParam(c, "version_id")
	if !ok {
		return
	}
	result, err := h.service.PublishFormVersion(c.Request.Context(), httpx.Claims(c).CompanyID, id)
	respond(c, result, err)
}
