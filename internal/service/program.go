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
	"gorm.io/gorm/clause"
)

func (s *Service) ListTreatmentPrograms(ctx context.Context, companyID uuid.UUID, page, pageSize int, status, category string) (*domain.Page[domain.TreatmentProgram], error) {
	base := s.db.WithContext(ctx).Model(&domain.TreatmentProgram{}).Where("company_id = ?", companyID)
	if status != "" {
		base = base.Where("status = ?", status)
	}
	if category != "" {
		base = base.Where("category = ?", strings.ToUpper(category))
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.TreatmentProgram
	if err := base.Preload("CurrentVersion").Order("name ASC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetTreatmentProgram(ctx context.Context, companyID, id uuid.UUID) (*domain.TreatmentProgramDetail, error) {
	var program domain.TreatmentProgram
	if err := s.db.WithContext(ctx).Preload("CurrentVersion").First(&program, "id = ? AND company_id = ?", id, companyID).Error; err != nil {
		return nil, translate(err, "treatment program")
	}
	var versions []domain.ProgramVersion
	if err := s.db.WithContext(ctx).Preload("JourneyWorkflow").Preload("StepTemplates", func(db *gorm.DB) *gorm.DB { return db.Order("display_order ASC") }).
		Preload("StepTemplates.Dependencies").Preload("StepTemplates.FormAssignments.FormVersion.FormTemplate").
		Where("program_id = ?", id).Order("version_number DESC").Find(&versions).Error; err != nil {
		return nil, err
	}
	var locations []domain.ProgramLocation
	if err := s.db.WithContext(ctx).Preload("Location").Where("program_id = ?", id).Find(&locations).Error; err != nil {
		return nil, err
	}
	return &domain.TreatmentProgramDetail{Program: program, Versions: versions, Locations: locations}, nil
}

func (s *Service) CreateTreatmentProgram(ctx context.Context, companyID uuid.UUID, req domain.TreatmentProgramRequest) (*domain.TreatmentProgram, error) {
	status := req.Status
	if status == "" {
		status = domain.StatusActive
	}
	program := domain.TreatmentProgram{CompanyID: companyID, Code: strings.ToUpper(strings.TrimSpace(req.Code)), Name: strings.TrimSpace(req.Name), Category: req.Category, Description: strings.TrimSpace(req.Description), Status: status}
	err := s.db.WithContext(ctx).Create(&program).Error
	return &program, translate(err, "treatment program")
}

func (s *Service) UpdateTreatmentProgram(ctx context.Context, companyID, id uuid.UUID, req domain.TreatmentProgramRequest) (*domain.TreatmentProgram, error) {
	updates := map[string]any{"code": strings.ToUpper(strings.TrimSpace(req.Code)), "name": strings.TrimSpace(req.Name), "category": req.Category, "description": strings.TrimSpace(req.Description)}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	result := s.db.WithContext(ctx).Model(&domain.TreatmentProgram{}).Where("id = ? AND company_id = ?", id, companyID).Updates(updates)
	if result.Error != nil {
		return nil, translate(result.Error, "treatment program")
	}
	if result.RowsAffected == 0 {
		return nil, httpx.NotFound("treatment program")
	}
	var program domain.TreatmentProgram
	err := s.db.WithContext(ctx).Preload("CurrentVersion").First(&program, "id = ?", id).Error
	return &program, err
}

func (s *Service) CreateProgramVersion(ctx context.Context, companyID, programID uuid.UUID, req domain.ProgramVersionRequest) (*domain.ProgramVersion, error) {
	var created domain.ProgramVersion
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var program domain.TreatmentProgram
		if err := tx.First(&program, "id = ? AND company_id = ?", programID, companyID).Error; err != nil {
			return translate(err, "treatment program")
		}
		if req.JourneyWorkflowID != nil {
			if err := validWorkflow(tx, companyID, *req.JourneyWorkflowID, "JOURNEY", true); err != nil {
				return err
			}
		} else if program.CurrentVersionID != nil {
			var current domain.ProgramVersion
			if err := tx.First(&current, "id = ?", *program.CurrentVersionID).Error; err != nil {
				return err
			}
			req.JourneyWorkflowID = current.JourneyWorkflowID
		}
		var maxVersion int
		if err := tx.Model(&domain.ProgramVersion{}).Where("program_id = ?", programID).Select("COALESCE(MAX(version_number), 0)").Scan(&maxVersion).Error; err != nil {
			return err
		}
		created = domain.ProgramVersion{ProgramID: programID, VersionNumber: maxVersion + 1, Status: domain.ConfigStatusDraft, JourneyWorkflowID: req.JourneyWorkflowID, ChangeNote: strings.TrimSpace(req.ChangeNote)}
		if err := tx.Create(&created).Error; err != nil {
			return translate(err, "program version")
		}
		if program.CurrentVersionID != nil {
			return cloneProgramVersion(tx, *program.CurrentVersionID, created.ID)
		}
		return nil
	})
	return &created, err
}

func (s *Service) PublishProgramVersion(ctx context.Context, companyID, versionID uuid.UUID) (*domain.ProgramVersion, error) {
	var version domain.ProgramVersion
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Preload("Program").First(&version, "id = ?", versionID).Error; err != nil {
			return translate(err, "program version")
		}
		if version.Program == nil || version.Program.CompanyID != companyID {
			return httpx.NotFound("program version")
		}
		if version.Status != domain.ConfigStatusDraft {
			return httpx.Conflict("only a draft program version can be published")
		}
		if err := validateProgramVersion(tx, companyID, &version); err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&domain.ProgramVersion{}).Where("program_id = ? AND status = ?", version.ProgramID, domain.ConfigStatusPublished).
			Updates(map[string]any{"status": domain.ConfigStatusRetired}).Error; err != nil {
			return err
		}
		if err := tx.Model(&version).Updates(map[string]any{"status": domain.ConfigStatusPublished, "published_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.TreatmentProgram{}).Where("id = ?", version.ProgramID).Update("current_version_id", version.ID).Error
	})
	return &version, err
}

func (s *Service) CreateStepTemplate(ctx context.Context, companyID, versionID uuid.UUID, req domain.StepTemplateRequest) (*domain.StepTemplate, error) {
	if err := ensureDraftProgramVersion(s.db.WithContext(ctx), companyID, versionID); err != nil {
		return nil, err
	}
	if req.AssignedRoleID != nil {
		if err := validRole(s.db.WithContext(ctx), *req.AssignedRoleID); err != nil {
			return nil, err
		}
	}
	if req.WorkflowTemplateID != nil {
		if err := validWorkflow(s.db.WithContext(ctx), companyID, *req.WorkflowTemplateID, "STEP", true); err != nil {
			return nil, err
		}
	}
	condition, err := marshalObject(req.Condition)
	if err != nil {
		return nil, err
	}
	required := true
	if req.IsRequired != nil {
		required = *req.IsRequired
	}
	repeatType := req.RepeatType
	if repeatType == "" {
		repeatType = "NONE"
	}
	if repeatType == "INTERVAL" && (req.RepeatIntervalDays == nil || req.MaxOccurrences == nil) {
		return nil, httpx.BadRequest("interval repeat requires repeat_interval_days and max_occurrences")
	}
	step := domain.StepTemplate{ProgramVersionID: versionID, Code: strings.ToLower(strings.TrimSpace(req.Code)), Name: strings.TrimSpace(req.Name), StepType: strings.ToUpper(strings.TrimSpace(req.StepType)), DisplayOrder: req.DisplayOrder, IsRequired: required, AssignedRoleID: req.AssignedRoleID, DueDays: req.DueDays, CreatesOrder: req.CreatesOrder, RepeatType: repeatType, RepeatIntervalDays: req.RepeatIntervalDays, MaxOccurrences: req.MaxOccurrences, Condition: condition, WorkflowTemplateID: req.WorkflowTemplateID}
	err = s.db.WithContext(ctx).Create(&step).Error
	return &step, translate(err, "step template")
}

func (s *Service) UpdateStepTemplate(ctx context.Context, companyID, id uuid.UUID, req domain.StepTemplateRequest) (*domain.StepTemplate, error) {
	var step domain.StepTemplate
	if err := s.db.WithContext(ctx).Preload("ProgramVersion.Program").First(&step, "id = ?", id).Error; err != nil {
		return nil, translate(err, "step template")
	}
	if step.ProgramVersion == nil || step.ProgramVersion.Program == nil || step.ProgramVersion.Program.CompanyID != companyID {
		return nil, httpx.NotFound("step template")
	}
	if step.ProgramVersion.Status != domain.ConfigStatusDraft {
		return nil, httpx.Conflict("published step templates are immutable; create a new program version")
	}
	if req.AssignedRoleID != nil {
		if err := validRole(s.db.WithContext(ctx), *req.AssignedRoleID); err != nil {
			return nil, err
		}
	}
	if req.WorkflowTemplateID != nil {
		if err := validWorkflow(s.db.WithContext(ctx), companyID, *req.WorkflowTemplateID, "STEP", true); err != nil {
			return nil, err
		}
	}
	condition, err := marshalObject(req.Condition)
	if err != nil {
		return nil, err
	}
	required := step.IsRequired
	if req.IsRequired != nil {
		required = *req.IsRequired
	}
	repeatType := req.RepeatType
	if repeatType == "" {
		repeatType = "NONE"
	}
	if repeatType == "INTERVAL" && (req.RepeatIntervalDays == nil || req.MaxOccurrences == nil) {
		return nil, httpx.BadRequest("interval repeat requires repeat_interval_days and max_occurrences")
	}
	updates := map[string]any{"code": strings.ToLower(strings.TrimSpace(req.Code)), "name": strings.TrimSpace(req.Name), "step_type": strings.ToUpper(strings.TrimSpace(req.StepType)), "display_order": req.DisplayOrder, "is_required": required, "assigned_role_id": req.AssignedRoleID, "due_days": req.DueDays, "creates_order": req.CreatesOrder, "repeat_type": repeatType, "repeat_interval_days": req.RepeatIntervalDays, "max_occurrences": req.MaxOccurrences, "condition": condition, "workflow_template_id": req.WorkflowTemplateID}
	if err := s.db.WithContext(ctx).Model(&step).Updates(updates).Error; err != nil {
		return nil, translate(err, "step template")
	}
	return &step, nil
}

func (s *Service) AddStepDependency(ctx context.Context, companyID, stepID uuid.UUID, req domain.StepDependencyRequest) (*domain.StepDependency, error) {
	var step, dependency domain.StepTemplate
	if err := s.db.WithContext(ctx).Preload("ProgramVersion.Program").First(&step, "id = ?", stepID).Error; err != nil {
		return nil, translate(err, "step template")
	}
	if step.ProgramVersion == nil || step.ProgramVersion.Program == nil || step.ProgramVersion.Program.CompanyID != companyID || step.ProgramVersion.Status != domain.ConfigStatusDraft {
		return nil, httpx.Conflict("dependencies can only be changed on a draft program version")
	}
	if err := s.db.WithContext(ctx).First(&dependency, "id = ? AND program_version_id = ?", req.DependsOnStepID, step.ProgramVersionID).Error; err != nil {
		return nil, httpx.BadRequest("dependency must belong to the same program version")
	}
	if step.ID == dependency.ID {
		return nil, httpx.BadRequest("a step cannot depend on itself")
	}
	condition, err := marshalObject(req.Condition)
	if err != nil {
		return nil, err
	}
	item := domain.StepDependency{StepTemplateID: step.ID, DependsOnStepID: dependency.ID, Condition: condition}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return nil, translate(err, "step dependency")
	}
	if err := validateDependencyGraph(s.db.WithContext(ctx), step.ProgramVersionID); err != nil {
		_ = s.db.WithContext(ctx).Delete(&item).Error
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpsertProgramLocation(ctx context.Context, companyID, programID uuid.UUID, req domain.ProgramLocationRequest) (*domain.ProgramLocation, error) {
	var program domain.TreatmentProgram
	if err := s.db.WithContext(ctx).First(&program, "id = ? AND company_id = ?", programID, companyID).Error; err != nil {
		return nil, translate(err, "treatment program")
	}
	if err := validLocation(s.db.WithContext(ctx), companyID, req.LocationID); err != nil {
		return nil, err
	}
	config, err := marshalObject(req.Config)
	if err != nil {
		return nil, err
	}
	status := req.Status
	if status == "" {
		status = domain.StatusActive
	}
	item := domain.ProgramLocation{ProgramID: programID, LocationID: req.LocationID, Status: status, Config: config}
	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "program_id"}, {Name: "location_id"}}, DoUpdates: clause.Assignments(map[string]any{"status": status, "config": config, "updated_at": time.Now()})}).Create(&item).Error
	return &item, err
}

func cloneProgramVersion(tx *gorm.DB, sourceID, targetID uuid.UUID) error {
	var sourceSteps []domain.StepTemplate
	if err := tx.Preload("Dependencies").Preload("FormAssignments").Where("program_version_id = ?", sourceID).Order("display_order").Find(&sourceSteps).Error; err != nil {
		return err
	}
	idMap := map[uuid.UUID]uuid.UUID{}
	for _, old := range sourceSteps {
		item := old
		item.BaseModel = domain.BaseModel{}
		item.ProgramVersionID = targetID
		item.Dependencies = nil
		item.FormAssignments = nil
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		idMap[old.ID] = item.ID
	}
	for _, old := range sourceSteps {
		for _, dep := range old.Dependencies {
			item := domain.StepDependency{StepTemplateID: idMap[old.ID], DependsOnStepID: idMap[dep.DependsOnStepID], Condition: dep.Condition}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		for _, assignment := range old.FormAssignments {
			item := domain.StepFormAssignment{StepTemplateID: idMap[old.ID], FormVersionID: assignment.FormVersionID, Phase: assignment.Phase, ActorType: assignment.ActorType, IsRequired: assignment.IsRequired}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureDraftProgramVersion(db *gorm.DB, companyID, versionID uuid.UUID) error {
	var version domain.ProgramVersion
	if err := db.Preload("Program").First(&version, "id = ?", versionID).Error; err != nil {
		return translate(err, "program version")
	}
	if version.Program == nil || version.Program.CompanyID != companyID {
		return httpx.NotFound("program version")
	}
	if version.Status != domain.ConfigStatusDraft {
		return httpx.Conflict("published program versions are immutable; create a new version")
	}
	return nil
}

func validateProgramVersion(tx *gorm.DB, companyID uuid.UUID, version *domain.ProgramVersion) error {
	var count int64
	if err := tx.Model(&domain.StepTemplate{}).Where("program_version_id = ?", version.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.BadRequest("program version must contain at least one step template")
	}
	if err := validateDependencyGraph(tx, version.ID); err != nil {
		return err
	}
	if version.JourneyWorkflowID != nil {
		if err := validWorkflow(tx, companyID, *version.JourneyWorkflowID, "JOURNEY", true); err != nil {
			return err
		}
	}
	var invalidForms int64
	if err := tx.Table("step_form_assignments sfa").Joins("JOIN step_templates st ON st.id = sfa.step_template_id").Joins("JOIN form_versions fv ON fv.id = sfa.form_version_id").Where("st.program_version_id = ? AND fv.status = ?", version.ID, domain.ConfigStatusDraft).Count(&invalidForms).Error; err != nil {
		return err
	}
	if invalidForms > 0 {
		return httpx.BadRequest("draft form versions cannot be assigned to a published program")
	}
	return nil
}

func validateDependencyGraph(db *gorm.DB, versionID uuid.UUID) error {
	var steps []domain.StepTemplate
	if err := db.Where("program_version_id = ?", versionID).Find(&steps).Error; err != nil {
		return err
	}
	var deps []domain.StepDependency
	if err := db.Where("step_template_id IN (SELECT id FROM step_templates WHERE program_version_id = ? AND deleted_at IS NULL)", versionID).Find(&deps).Error; err != nil {
		return err
	}
	graph := map[uuid.UUID][]uuid.UUID{}
	for _, dep := range deps {
		graph[dep.StepTemplateID] = append(graph[dep.StepTemplateID], dep.DependsOnStepID)
	}
	visiting, visited := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	var visit func(uuid.UUID) bool
	visit = func(id uuid.UUID) bool {
		if visiting[id] {
			return false
		}
		if visited[id] {
			return true
		}
		visiting[id] = true
		for _, dep := range graph[id] {
			if !visit(dep) {
				return false
			}
		}
		visiting[id] = false
		visited[id] = true
		return true
	}
	for _, step := range steps {
		if !visit(step.ID) {
			return httpx.BadRequest("step dependencies contain a cycle")
		}
	}
	return nil
}

func validRole(db *gorm.DB, id uuid.UUID) error {
	var count int64
	if err := db.Model(&domain.Role{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.NotFound("role")
	}
	return nil
}

func validWorkflow(db *gorm.DB, companyID, id uuid.UUID, entityType string, requirePublished bool) error {
	query := db.Model(&domain.WorkflowTemplate{}).Where("id = ? AND company_id = ? AND entity_type = ?", id, companyID, entityType)
	if requirePublished {
		query = query.Where("status = ?", domain.ConfigStatusPublished)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.NotFound("published workflow")
	}
	return nil
}

func marshalObject(value map[string]any) (datatypes.JSON, error) {
	if value == nil {
		return datatypes.JSON([]byte("{}")), nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, httpx.BadRequest("invalid JSON configuration")
	}
	return datatypes.JSON(data), nil
}
