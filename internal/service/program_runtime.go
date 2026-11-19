package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) InstantiateTreatmentProgram(ctx context.Context, claims *platformjwt.Claims, programID uuid.UUID, req domain.InstantiateProgramRequest) (*domain.CareJourney, error) {
	if err := locationAllowed(s.db.WithContext(ctx), claims, req.LocationID); err != nil {
		return nil, err
	}
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, req.PatientID); err != nil {
		return nil, err
	}
	if err := patientAtLocation(s.db.WithContext(ctx), req.PatientID, req.LocationID); err != nil {
		return nil, err
	}
	contextJSON, err := marshalObject(req.Context)
	if err != nil {
		return nil, err
	}
	startedAt := time.Now()
	if req.StartedAt != nil {
		startedAt = *req.StartedAt
	}
	var journey domain.CareJourney
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var program domain.TreatmentProgram
		if err := tx.Preload("CurrentVersion.JourneyWorkflow.States").First(&program, "id = ? AND company_id = ? AND status = ?", programID, claims.CompanyID, domain.StatusActive).Error; err != nil {
			return translate(err, "active treatment program")
		}
		if program.CurrentVersion == nil || program.CurrentVersion.Status != domain.ConfigStatusPublished {
			return httpx.Conflict("treatment program has no published version")
		}
		if err := ensureProgramAvailableAtLocation(tx, program.ID, req.LocationID); err != nil {
			return err
		}
		nextEpisode, err := nextJourneyEpisode(tx, claims.CompanyID, req.PatientID, program.Code)
		if err != nil {
			return err
		}
		status := domain.JourneyStatusOnboarding
		if program.CurrentVersion.JourneyWorkflow != nil {
			for _, state := range program.CurrentVersion.JourneyWorkflow.States {
				if state.IsInitial {
					status = state.StandardCode
					break
				}
			}
		}
		versionID := program.CurrentVersion.ID
		journey = domain.CareJourney{CompanyID: claims.CompanyID, ProgramVersionID: &versionID, PatientID: req.PatientID, LocationID: req.LocationID, CreatedByMembershipID: claims.MembershipID, ProgramCode: program.Code, EpisodeNumber: nextEpisode, Status: status, CurrentStage: "NOT_STARTED", CurrentCycle: 1, ContextData: contextJSON, StartedAt: startedAt}
		if err := tx.Create(&journey).Error; err != nil {
			return translate(err, "care journey")
		}
		var templates []domain.StepTemplate
		if err := tx.Preload("Dependencies").Where("program_version_id = ?", versionID).Order("display_order ASC").Find(&templates).Error; err != nil {
			return err
		}
		firstStage := ""
		for _, template := range templates {
			conditionOK := evaluateCondition(template.Condition, req.Context)
			status := domain.StepStatusNotStarted
			activeDependencies := 0
			for _, dependency := range template.Dependencies {
				if evaluateCondition(dependency.Condition, req.Context) {
					activeDependencies++
				}
			}
			if !conditionOK {
				status = domain.StepStatusSkipped
			} else if activeDependencies == 0 {
				status = domain.StepStatusReady
			}
			metadata, _ := json.Marshal(map[string]any{"template_name": template.Name, "is_required": template.IsRequired, "creates_order": template.CreatesOrder, "assigned_role_id": template.AssignedRoleID, "workflow_template_id": template.WorkflowTemplateID})
			var dueAt *time.Time
			if status == domain.StepStatusReady && template.DueDays != nil {
				due := startedAt.Add(time.Duration(*template.DueDays) * 24 * time.Hour)
				dueAt = &due
			}
			templateID := template.ID
			step := domain.JourneyStep{CompanyID: claims.CompanyID, StepTemplateID: &templateID, JourneyID: journey.ID, StepKey: template.Code, StepType: template.StepType, CycleNumber: 1, TemplateIteration: 1, Status: status, DueAt: dueAt, Metadata: datatypes.JSON(metadata)}
			if status == domain.StepStatusSkipped {
				now := time.Now()
				step.CompletedAt = &now
			}
			if err := tx.Create(&step).Error; err != nil {
				return err
			}
			if firstStage == "" && status == domain.StepStatusReady {
				firstStage = template.Code
			}
		}
		if firstStage != "" {
			return tx.Model(&journey).Update("current_stage", strings.ToUpper(firstStage)).Error
		}
		return nil
	})
	return &journey, err
}

func ensureProgramAvailableAtLocation(db *gorm.DB, programID, locationID uuid.UUID) error {
	var configured int64
	if err := db.Model(&domain.ProgramLocation{}).Where("program_id = ?", programID).Count(&configured).Error; err != nil {
		return err
	}
	if configured == 0 {
		return nil
	}
	var active int64
	if err := db.Model(&domain.ProgramLocation{}).Where("program_id = ? AND location_id = ? AND status = ?", programID, locationID, domain.StatusActive).Count(&active).Error; err != nil {
		return err
	}
	if active == 0 {
		return httpx.Forbidden("treatment program is not enabled at this location")
	}
	return nil
}

func validateStepStatusTransition(db *gorm.DB, claims *platformjwt.Claims, step *domain.JourneyStep, target string) error {
	if step.StepTemplateID == nil {
		return nil
	}
	var template domain.StepTemplate
	if err := db.First(&template, "id = ?", *step.StepTemplateID).Error; err != nil {
		return translate(err, "step template")
	}
	if template.AssignedRoleID != nil && claims.RoleCode != "ADMIN" {
		var role domain.Role
		if err := db.First(&role, "id = ?", *template.AssignedRoleID).Error; err != nil {
			return translate(err, "assigned role")
		}
		if role.Code != claims.RoleCode {
			return httpx.Forbidden("your role is not assigned to this treatment step")
		}
	}
	if template.WorkflowTemplateID == nil {
		return nil
	}
	return validateConfiguredTransition(db, claims, *template.WorkflowTemplateID, step.Status, target)
}

func validateJourneyStatusTransition(db *gorm.DB, claims *platformjwt.Claims, journey *domain.CareJourney, target string) error {
	if journey.ProgramVersionID == nil {
		return nil
	}
	var version domain.ProgramVersion
	if err := db.First(&version, "id = ?", *journey.ProgramVersionID).Error; err != nil {
		return err
	}
	if version.JourneyWorkflowID == nil {
		return nil
	}
	return validateConfiguredTransition(db, claims, *version.JourneyWorkflowID, journey.Status, target)
}

func validateConfiguredTransition(db *gorm.DB, claims *platformjwt.Claims, workflowID uuid.UUID, from, to string) error {
	if from == to {
		return nil
	}
	var transition domain.WorkflowTransition
	if err := db.Preload("RequiredRole").Where("workflow_template_id = ? AND from_state_code = ? AND to_state_code = ?", workflowID, from, to).First(&transition).Error; err != nil {
		return httpx.Conflict(fmt.Sprintf("workflow does not allow transition %s -> %s", from, to))
	}
	if transition.RequiredRole != nil && claims.RoleCode != "ADMIN" && transition.RequiredRole.Code != claims.RoleCode {
		return httpx.Forbidden("your role cannot perform this status transition")
	}
	return nil
}

func recordStatusHistory(tx *gorm.DB, claims *platformjwt.Claims, entityType string, entityID uuid.UUID, from, to, note string) error {
	if from == to {
		return nil
	}
	item := domain.StatusHistory{CompanyID: claims.CompanyID, EntityType: entityType, EntityID: entityID, FromStatus: from, ToStatus: to, PerformedByMembershipID: claims.MembershipID, Note: strings.TrimSpace(note), CreatedAt: time.Now()}
	return tx.Create(&item).Error
}

func validateRequiredForms(db *gorm.DB, step *domain.JourneyStep) error {
	if step.StepTemplateID == nil {
		return nil
	}
	var assignments []domain.StepFormAssignment
	if err := db.Where("step_template_id = ? AND is_required = true AND phase <> 'AFTER'", *step.StepTemplateID).Find(&assignments).Error; err != nil {
		return err
	}
	for _, assignment := range assignments {
		var count int64
		if err := db.Model(&domain.IntakeSubmission{}).Where("journey_step_id = ? AND form_version_id = ?", step.ID, assignment.FormVersionID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return httpx.Conflict("required form has not been submitted for this step")
		}
	}
	return nil
}

func advanceConfiguredSteps(tx *gorm.DB, claims *platformjwt.Claims, completed *domain.JourneyStep) error {
	if completed.StepTemplateID == nil {
		return nil
	}
	var template domain.StepTemplate
	if err := tx.First(&template, "id = ?", *completed.StepTemplateID).Error; err != nil {
		return err
	}
	spawned, err := spawnRepeatingStep(tx, claims, completed, &template)
	if err != nil || spawned {
		return err
	}
	var dependents []domain.StepDependency
	if err := tx.Where("depends_on_step_id = ?", template.ID).Find(&dependents).Error; err != nil {
		return err
	}
	for _, relation := range dependents {
		var candidate domain.JourneyStep
		if err := tx.Where("journey_id = ? AND step_template_id = ? AND status IN ?", completed.JourneyID, relation.StepTemplateID, []string{domain.StepStatusNotStarted, domain.StepStatusBlocked}).First(&candidate).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}
		ready, err := dependenciesCompleted(tx, completed.JourneyID, relation.StepTemplateID)
		if err != nil {
			return err
		}
		if !ready {
			continue
		}
		var candidateTemplate domain.StepTemplate
		if err := tx.First(&candidateTemplate, "id = ?", relation.StepTemplateID).Error; err != nil {
			return err
		}
		var journey domain.CareJourney
		if err := tx.First(&journey, "id = ?", completed.JourneyID).Error; err != nil {
			return err
		}
		var contextData map[string]any
		_ = json.Unmarshal(journey.ContextData, &contextData)
		status := domain.StepStatusReady
		if !evaluateCondition(candidateTemplate.Condition, contextData) {
			status = domain.StepStatusSkipped
		}
		updates := map[string]any{"status": status}
		if status == domain.StepStatusReady && candidateTemplate.DueDays != nil {
			updates["due_at"] = time.Now().Add(time.Duration(*candidateTemplate.DueDays) * 24 * time.Hour)
		}
		if status == domain.StepStatusSkipped {
			updates["completed_at"] = time.Now()
		}
		previousStatus := candidate.Status
		if err := tx.Model(&candidate).Updates(updates).Error; err != nil {
			return err
		}
		if err := recordStatusHistory(tx, claims, "STEP", candidate.ID, previousStatus, status, "Automatically advanced by dependency completion"); err != nil {
			return err
		}
	}
	return nil
}

func spawnRepeatingStep(tx *gorm.DB, claims *platformjwt.Claims, completed *domain.JourneyStep, template *domain.StepTemplate) (bool, error) {
	if template.RepeatType != "INTERVAL" || template.RepeatIntervalDays == nil || template.MaxOccurrences == nil || completed.TemplateIteration >= *template.MaxOccurrences {
		return false, nil
	}
	nextIteration := completed.TemplateIteration + 1
	var count int64
	if err := tx.Model(&domain.JourneyStep{}).Where("journey_id = ? AND step_template_id = ? AND template_iteration = ?", completed.JourneyID, template.ID, nextIteration).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	due := time.Now().Add(time.Duration(*template.RepeatIntervalDays) * 24 * time.Hour)
	templateID := template.ID
	step := domain.JourneyStep{CompanyID: claims.CompanyID, StepTemplateID: &templateID, JourneyID: completed.JourneyID, StepKey: fmt.Sprintf("%s-%d", template.Code, nextIteration), StepType: template.StepType, CycleNumber: completed.CycleNumber, TemplateIteration: nextIteration, Status: domain.StepStatusReady, DueAt: &due, Metadata: completed.Metadata}
	if err := tx.Create(&step).Error; err != nil {
		return false, err
	}
	return true, recordStatusHistory(tx, claims, "STEP", step.ID, domain.StepStatusNotStarted, domain.StepStatusReady, "Automatically created recurring step")
}

func dependenciesCompleted(db *gorm.DB, journeyID, templateID uuid.UUID) (bool, error) {
	var deps []domain.StepDependency
	if err := db.Where("step_template_id = ?", templateID).Find(&deps).Error; err != nil {
		return false, err
	}
	var journey domain.CareJourney
	if err := db.First(&journey, "id = ?", journeyID).Error; err != nil {
		return false, err
	}
	contextData := map[string]any{}
	_ = json.Unmarshal(journey.ContextData, &contextData)
	for _, dep := range deps {
		if !evaluateCondition(dep.Condition, contextData) {
			continue
		}
		var latest domain.JourneyStep
		if err := db.Where("journey_id = ? AND step_template_id = ?", journeyID, dep.DependsOnStepID).Order("template_iteration DESC").First(&latest).Error; err != nil {
			return false, err
		}
		if latest.Status != domain.StepStatusCompleted && latest.Status != domain.StepStatusSkipped {
			return false, nil
		}
	}
	return true, nil
}

func evaluateCondition(raw datatypes.JSON, context map[string]any) bool {
	if len(raw) == 0 || string(raw) == "{}" {
		return true
	}
	if context == nil {
		context = map[string]any{}
	}
	var condition struct {
		Field    string `json:"field"`
		Operator string `json:"operator"`
		Value    any    `json:"value"`
	}
	if err := json.Unmarshal(raw, &condition); err != nil || condition.Field == "" {
		return true
	}
	actual, exists := nestedValue(context, condition.Field)
	switch condition.Operator {
	case "exists":
		return exists
	case "equals":
		return exists && reflect.DeepEqual(actual, condition.Value)
	case "not_equals":
		return !exists || !reflect.DeepEqual(actual, condition.Value)
	case "in":
		values, ok := condition.Value.([]any)
		if !ok {
			return false
		}
		for _, value := range values {
			if reflect.DeepEqual(actual, value) {
				return true
			}
		}
	}
	return false
}

func nestedValue(context map[string]any, path string) (any, bool) {
	var current any = context
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
