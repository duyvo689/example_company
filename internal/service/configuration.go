package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) ListWorkflows(ctx context.Context, companyID uuid.UUID) ([]domain.WorkflowTemplate, error) {
	var items []domain.WorkflowTemplate
	err := s.db.WithContext(ctx).Preload("States", func(db *gorm.DB) *gorm.DB { return db.Order("display_order") }).Preload("Transitions.RequiredRole").Where("company_id = ?", companyID).Order("name").Find(&items).Error
	return items, err
}

func (s *Service) CreateWorkflow(ctx context.Context, companyID uuid.UUID, req domain.WorkflowTemplateRequest) (*domain.WorkflowTemplate, error) {
	if err := validateWorkflowRequest(req); err != nil {
		return nil, err
	}
	workflow := domain.WorkflowTemplate{CompanyID: companyID, Code: strings.ToUpper(strings.TrimSpace(req.Code)), Name: strings.TrimSpace(req.Name), EntityType: req.EntityType, Status: domain.ConfigStatusDraft}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&workflow).Error; err != nil {
			return translate(err, "workflow")
		}
		for _, stateReq := range req.States {
			state := domain.WorkflowState{WorkflowTemplateID: workflow.ID, Code: strings.ToUpper(stateReq.Code), StandardCode: strings.ToUpper(stateReq.StandardCode), DisplayName: strings.TrimSpace(stateReq.DisplayName), DisplayOrder: stateReq.DisplayOrder, IsInitial: stateReq.IsInitial, IsTerminal: stateReq.IsTerminal}
			if err := tx.Create(&state).Error; err != nil {
				return translate(err, "workflow state")
			}
		}
		for _, transitionReq := range req.Transitions {
			if transitionReq.RequiredRoleID != nil {
				if err := validRole(tx, *transitionReq.RequiredRoleID); err != nil {
					return err
				}
			}
			transition := domain.WorkflowTransition{WorkflowTemplateID: workflow.ID, FromStateCode: strings.ToUpper(transitionReq.FromStateCode), ToStateCode: strings.ToUpper(transitionReq.ToStateCode), RequiredRoleID: transitionReq.RequiredRoleID}
			if err := tx.Create(&transition).Error; err != nil {
				return translate(err, "workflow transition")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.getWorkflow(ctx, companyID, workflow.ID)
}

func (s *Service) PublishWorkflow(ctx context.Context, companyID, id uuid.UUID) (*domain.WorkflowTemplate, error) {
	workflow, err := s.getWorkflow(ctx, companyID, id)
	if err != nil {
		return nil, err
	}
	if workflow.Status != domain.ConfigStatusDraft {
		return nil, httpx.Conflict("only a draft workflow can be published")
	}
	if err := validateWorkflowGraph(workflow); err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(workflow).Update("status", domain.ConfigStatusPublished).Error; err != nil {
		return nil, err
	}
	workflow.Status = domain.ConfigStatusPublished
	return workflow, nil
}

func (s *Service) CreateFormTemplate(ctx context.Context, companyID uuid.UUID, req domain.FormTemplateRequest) (*domain.FormTemplate, error) {
	status := req.Status
	if status == "" {
		status = domain.StatusActive
	}
	form := domain.FormTemplate{CompanyID: companyID, Code: strings.ToUpper(strings.TrimSpace(req.Code)), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Status: status}
	err := s.db.WithContext(ctx).Create(&form).Error
	return &form, translate(err, "form template")
}

func (s *Service) ListFormTemplates(ctx context.Context, companyID uuid.UUID) ([]domain.FormTemplate, error) {
	var items []domain.FormTemplate
	err := s.db.WithContext(ctx).Preload("CurrentVersion").Where("company_id = ?", companyID).Order("name").Find(&items).Error
	return items, err
}

func (s *Service) CreateFormVersion(ctx context.Context, companyID, formID uuid.UUID, req domain.FormVersionRequest) (*domain.FormVersion, error) {
	if containsUnsafeSchemaKey(req.Schema) {
		return nil, httpx.BadRequest("form schema cannot contain executable code")
	}
	var form domain.FormTemplate
	if err := s.db.WithContext(ctx).First(&form, "id = ? AND company_id = ?", formID, companyID).Error; err != nil {
		return nil, translate(err, "form template")
	}
	schema, err := marshalObject(req.Schema)
	if err != nil {
		return nil, err
	}
	var maxVersion int
	if err := s.db.WithContext(ctx).Model(&domain.FormVersion{}).Where("form_template_id = ?", formID).Select("COALESCE(MAX(version_number), 0)").Scan(&maxVersion).Error; err != nil {
		return nil, err
	}
	version := domain.FormVersion{FormTemplateID: formID, VersionNumber: maxVersion + 1, Status: domain.ConfigStatusDraft, Schema: schema}
	err = s.db.WithContext(ctx).Create(&version).Error
	return &version, translate(err, "form version")
}

func (s *Service) PublishFormVersion(ctx context.Context, companyID, versionID uuid.UUID) (*domain.FormVersion, error) {
	var version domain.FormVersion
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Preload("FormTemplate").First(&version, "id = ?", versionID).Error; err != nil {
			return translate(err, "form version")
		}
		if version.FormTemplate == nil || version.FormTemplate.CompanyID != companyID {
			return httpx.NotFound("form version")
		}
		if version.Status != domain.ConfigStatusDraft {
			return httpx.Conflict("only a draft form version can be published")
		}
		now := time.Now()
		if err := tx.Model(&domain.FormVersion{}).Where("form_template_id = ? AND status = ?", version.FormTemplateID, domain.ConfigStatusPublished).Update("status", domain.ConfigStatusRetired).Error; err != nil {
			return err
		}
		if err := tx.Model(&version).Updates(map[string]any{"status": domain.ConfigStatusPublished, "published_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.FormTemplate{}).Where("id = ?", version.FormTemplateID).Update("current_version_id", version.ID).Error
	})
	return &version, err
}

func (s *Service) AssignFormToStep(ctx context.Context, companyID, stepID uuid.UUID, req domain.StepFormAssignmentRequest) (*domain.StepFormAssignment, error) {
	var step domain.StepTemplate
	if err := s.db.WithContext(ctx).Preload("ProgramVersion.Program").First(&step, "id = ?", stepID).Error; err != nil {
		return nil, translate(err, "step template")
	}
	if step.ProgramVersion == nil || step.ProgramVersion.Program == nil || step.ProgramVersion.Program.CompanyID != companyID || step.ProgramVersion.Status != domain.ConfigStatusDraft {
		return nil, httpx.Conflict("forms can only be assigned to a draft program version")
	}
	var formVersion domain.FormVersion
	if err := s.db.WithContext(ctx).Preload("FormTemplate").First(&formVersion, "id = ? AND status = ?", req.FormVersionID, domain.ConfigStatusPublished).Error; err != nil {
		return nil, httpx.NotFound("published form version")
	}
	if formVersion.FormTemplate == nil || formVersion.FormTemplate.CompanyID != companyID {
		return nil, httpx.NotFound("published form version")
	}
	required := true
	if req.IsRequired != nil {
		required = *req.IsRequired
	}
	assignment := domain.StepFormAssignment{StepTemplateID: stepID, FormVersionID: req.FormVersionID, Phase: req.Phase, ActorType: req.ActorType, IsRequired: required}
	err := s.db.WithContext(ctx).Create(&assignment).Error
	return &assignment, translate(err, "step form assignment")
}

func (s *Service) getWorkflow(ctx context.Context, companyID, id uuid.UUID) (*domain.WorkflowTemplate, error) {
	var workflow domain.WorkflowTemplate
	err := s.db.WithContext(ctx).Preload("States", func(db *gorm.DB) *gorm.DB { return db.Order("display_order") }).Preload("Transitions.RequiredRole").First(&workflow, "id = ? AND company_id = ?", id, companyID).Error
	return &workflow, translate(err, "workflow")
}

func validateWorkflowRequest(req domain.WorkflowTemplateRequest) error {
	initial := 0
	states := map[string]struct{}{}
	allowedJourney := map[string]bool{"DRAFT": true, "ONBOARDING": true, "ACTIVE": true, "PAUSED": true, "COMPLETED": true, "CHURNED": true, "CANCELLED": true}
	allowedStep := map[string]bool{"NOT_STARTED": true, "READY": true, "WAITING_PATIENT": true, "IN_PROGRESS": true, "WAITING_DOCTOR": true, "BLOCKED": true, "COMPLETED": true, "SKIPPED": true, "CANCELLED": true}
	for _, state := range req.States {
		code, standard := strings.ToUpper(state.Code), strings.ToUpper(state.StandardCode)
		if code != standard {
			return httpx.BadRequest("workflow state code must equal its standard code; customize display_name instead")
		}
		if req.EntityType == "JOURNEY" && !allowedJourney[standard] {
			return httpx.BadRequest("unsupported standard journey status: " + standard)
		}
		if req.EntityType == "STEP" && !allowedStep[standard] {
			return httpx.BadRequest("unsupported standard step status: " + standard)
		}
		if _, exists := states[code]; exists {
			return httpx.BadRequest("duplicate workflow state: " + code)
		}
		states[code] = struct{}{}
		if state.IsInitial {
			initial++
		}
	}
	if initial != 1 {
		return httpx.BadRequest("workflow must have exactly one initial state")
	}
	for _, transition := range req.Transitions {
		if _, ok := states[strings.ToUpper(transition.FromStateCode)]; !ok {
			return httpx.BadRequest("transition references an unknown from state")
		}
		if _, ok := states[strings.ToUpper(transition.ToStateCode)]; !ok {
			return httpx.BadRequest("transition references an unknown to state")
		}
	}
	return nil
}

func validateWorkflowGraph(workflow *domain.WorkflowTemplate) error {
	if len(workflow.States) < 2 || len(workflow.Transitions) == 0 {
		return httpx.BadRequest("workflow requires states and transitions")
	}
	reachable := map[string]bool{}
	queue := []string{}
	for _, state := range workflow.States {
		if state.IsInitial {
			queue = append(queue, state.Code)
			reachable[state.Code] = true
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, transition := range workflow.Transitions {
			if transition.FromStateCode == current && !reachable[transition.ToStateCode] {
				reachable[transition.ToStateCode] = true
				queue = append(queue, transition.ToStateCode)
			}
		}
	}
	for _, state := range workflow.States {
		if !reachable[state.Code] {
			return httpx.BadRequest("workflow contains unreachable state: " + state.Code)
		}
	}
	return nil
}

func containsUnsafeSchemaKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if lower == "script" || lower == "eval" || lower == "javascript" || lower == "executable" {
				return true
			}
			if containsUnsafeSchemaKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsUnsafeSchemaKey(child) {
				return true
			}
		}
	}
	return false
}

func validateFormAnswers(schema datatypes.JSON, answers map[string]any) error {
	var definition struct {
		Fields []struct {
			Key      string `json:"key"`
			Type     string `json:"type"`
			Required bool   `json:"required"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(schema, &definition); err != nil {
		return httpx.BadRequest("invalid stored form schema")
	}
	for _, field := range definition.Fields {
		value, exists := answers[field.Key]
		if field.Required && (!exists || value == nil || value == "") {
			return httpx.BadRequest("missing required form field: " + field.Key)
		}
		if !exists || value == nil {
			continue
		}
		switch strings.ToLower(field.Type) {
		case "number":
			if _, ok := value.(float64); !ok {
				return httpx.BadRequest("form field must be a number: " + field.Key)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return httpx.BadRequest("form field must be a boolean: " + field.Key)
			}
		case "text", "date", "single_select", "signature", "file":
			if _, ok := value.(string); !ok {
				return httpx.BadRequest("form field must be text: " + field.Key)
			}
		case "multi_select":
			if _, ok := value.([]any); !ok {
				return httpx.BadRequest("form field must be an array: " + field.Key)
			}
		}
	}
	return nil
}
