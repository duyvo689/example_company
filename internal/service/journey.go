package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) ListCareJourneys(ctx context.Context, claims *platformjwt.Claims, page, pageSize int, status, program string, patientID, locationID *uuid.UUID) (*domain.Page[domain.CareJourney], error) {
	base := s.db.WithContext(ctx).Model(&domain.CareJourney{}).Where("care_journeys.company_id = ?", claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		base = base.Joins("JOIN location_assignments la ON la.location_id = care_journeys.location_id AND la.membership_id = ?", claims.MembershipID)
	}
	if status != "" {
		base = base.Where("care_journeys.status = ?", strings.ToUpper(status))
	}
	if program != "" {
		base = base.Where("care_journeys.program_code = ?", strings.ToUpper(program))
	}
	if patientID != nil {
		base = base.Where("care_journeys.patient_id = ?", *patientID)
	}
	if locationID != nil {
		if err := locationAllowed(s.db.WithContext(ctx), claims, *locationID); err != nil {
			return nil, err
		}
		base = base.Where("care_journeys.location_id = ?", *locationID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.CareJourney
	if err := base.Preload("Patient").Preload("Location").Order("care_journeys.created_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetCareJourney(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID) (*domain.CareJourneyDetail, error) {
	if err := journeyAllowed(s.db.WithContext(ctx), claims, id); err != nil {
		return nil, err
	}
	var journey domain.CareJourney
	if err := s.db.WithContext(ctx).Preload("Patient").Preload("Location").Preload("CreatedBy.User").Preload("ProgramVersion.JourneyWorkflow").First(&journey, "id = ? AND company_id = ?", id, claims.CompanyID).Error; err != nil {
		return nil, translate(err, "care journey")
	}
	var steps []domain.JourneyStep
	if err := s.db.WithContext(ctx).Preload("AssignedMembership.User").Preload("StepTemplate.FormAssignments.FormVersion.FormTemplate").Where("journey_id = ?", id).Order("cycle_number ASC, created_at ASC").Find(&steps).Error; err != nil {
		return nil, err
	}
	var events []domain.PatientChurnEvent
	if err := s.db.WithContext(ctx).Preload("PerformedBy.User").Where("journey_id = ?", id).Order("occurred_at DESC").Find(&events).Error; err != nil {
		return nil, err
	}
	var history []domain.StatusHistory
	if err := s.db.WithContext(ctx).
		Where("company_id = ? AND ((entity_type = 'JOURNEY' AND entity_id = ?) OR (entity_type = 'STEP' AND entity_id IN (SELECT id FROM journey_steps WHERE journey_id = ?)))", claims.CompanyID, id, id).
		Order("created_at DESC").Find(&history).Error; err != nil {
		return nil, err
	}
	return &domain.CareJourneyDetail{Journey: journey, Steps: steps, ChurnEvents: events, StatusHistory: history}, nil
}

func (s *Service) CreateCareJourney(ctx context.Context, claims *platformjwt.Claims, req domain.CreateCareJourneyRequest) (*domain.CareJourney, error) {
	if err := locationAllowed(s.db.WithContext(ctx), claims, req.LocationID); err != nil {
		return nil, err
	}
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, req.PatientID); err != nil {
		return nil, err
	}
	if err := patientAtLocation(s.db.WithContext(ctx), req.PatientID, req.LocationID); err != nil {
		return nil, err
	}
	status := req.Status
	if status == "" {
		status = domain.JourneyStatusOnboarding
	}
	stage := strings.ToUpper(strings.TrimSpace(req.CurrentStage))
	if stage == "" {
		stage = "WAITING_INTAKE"
	}
	cycle := req.CurrentCycle
	if cycle == 0 {
		cycle = 1
	}
	startedAt := time.Now()
	if req.StartedAt != nil {
		startedAt = *req.StartedAt
	}
	journey := domain.CareJourney{
		CompanyID: claims.CompanyID, PatientID: req.PatientID, LocationID: req.LocationID,
		CreatedByMembershipID: claims.MembershipID, ProgramCode: strings.ToUpper(strings.TrimSpace(req.ProgramCode)),
		EpisodeNumber: req.EpisodeNumber, Status: status, CurrentStage: stage, CurrentCycle: cycle, StartedAt: startedAt,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if journey.EpisodeNumber == 0 {
			nextEpisode, err := nextJourneyEpisode(tx, claims.CompanyID, req.PatientID, journey.ProgramCode)
			if err != nil {
				return err
			}
			journey.EpisodeNumber = nextEpisode
		}
		if err := tx.Create(&journey).Error; err != nil {
			return translate(err, "care journey")
		}
		for _, stepReq := range req.InitialSteps {
			if _, err := createJourneyStep(tx, claims.CompanyID, journey.ID, stepReq); err != nil {
				return err
			}
		}
		return nil
	})
	return &journey, err
}

func (s *Service) UpdateCareJourney(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID, req domain.UpdateCareJourneyRequest) (*domain.CareJourney, error) {
	if err := journeyAllowed(s.db.WithContext(ctx), claims, id); err != nil {
		return nil, err
	}
	var current domain.CareJourney
	if err := s.db.WithContext(ctx).First(&current, "id = ? AND company_id = ?", id, claims.CompanyID).Error; err != nil {
		return nil, translate(err, "care journey")
	}
	updates := map[string]any{}
	if req.LocationID != nil {
		if err := locationAllowed(s.db.WithContext(ctx), claims, *req.LocationID); err != nil {
			return nil, err
		}
		if err := patientAtLocation(s.db.WithContext(ctx), current.PatientID, *req.LocationID); err != nil {
			return nil, err
		}
		updates["location_id"] = *req.LocationID
	}
	if req.Status != nil {
		if isTerminalJourneyStatus(*req.Status) {
			return nil, httpx.BadRequest("terminal journey statuses must be recorded through a churn event")
		}
		if err := validateJourneyStatusTransition(s.db.WithContext(ctx), claims, &current, *req.Status); err != nil {
			return nil, err
		}
		updates["status"] = *req.Status
		updates["ended_at"] = nil
	}
	if req.CurrentStage != nil {
		updates["current_stage"] = strings.ToUpper(strings.TrimSpace(*req.CurrentStage))
	}
	if req.CurrentCycle != nil {
		updates["current_cycle"] = *req.CurrentCycle
	}
	previousStatus := current.Status
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&current).Updates(updates).Error; err != nil {
				return translate(err, "care journey")
			}
		}
		if req.Status != nil {
			return recordStatusHistory(tx, claims, "JOURNEY", current.ID, previousStatus, *req.Status, "")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var journey domain.CareJourney
	err = s.db.WithContext(ctx).Preload("Patient").Preload("Location").Preload("ProgramVersion").First(&journey, "id = ?", id).Error
	return &journey, translate(err, "care journey")
}

func (s *Service) CreateJourneyStep(ctx context.Context, claims *platformjwt.Claims, journeyID uuid.UUID, req domain.JourneyStepRequest) (*domain.JourneyStep, error) {
	if err := journeyAllowed(s.db.WithContext(ctx), claims, journeyID); err != nil {
		return nil, err
	}
	return createJourneyStep(s.db.WithContext(ctx), claims.CompanyID, journeyID, req)
}

func (s *Service) UpdateJourneyStep(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID, req domain.UpdateJourneyStepRequest) (*domain.JourneyStep, error) {
	var step domain.JourneyStep
	if err := s.db.WithContext(ctx).First(&step, "id = ? AND company_id = ?", id, claims.CompanyID).Error; err != nil {
		return nil, translate(err, "journey step")
	}
	if err := journeyAllowed(s.db.WithContext(ctx), claims, step.JourneyID); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	previousStatus := step.Status
	if req.AssignedMembershipID != nil {
		if err := validMembership(s.db.WithContext(ctx), claims.CompanyID, *req.AssignedMembershipID); err != nil {
			return nil, err
		}
		updates["assigned_membership_id"] = *req.AssignedMembershipID
	}
	if req.Status != nil {
		if err := validateStepStatusTransition(s.db.WithContext(ctx), claims, &step, *req.Status); err != nil {
			return nil, err
		}
		if *req.Status == domain.StepStatusCompleted {
			if err := validateRequiredForms(s.db.WithContext(ctx), &step); err != nil {
				return nil, err
			}
		}
		updates["status"] = *req.Status
		if *req.Status == domain.StepStatusCompleted {
			updates["completed_at"] = time.Now()
		} else {
			updates["completed_at"] = nil
		}
	}
	if req.DueAt != nil {
		updates["due_at"] = *req.DueAt
	}
	if req.Metadata != nil {
		metadata, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, httpx.BadRequest("invalid metadata")
		}
		updates["metadata"] = datatypes.JSON(metadata)
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&step).Updates(updates).Error; err != nil {
				return err
			}
		}
		if req.JourneyStage != nil {
			if err := tx.Model(&domain.CareJourney{}).Where("id = ?", step.JourneyID).Update("current_stage", strings.ToUpper(strings.TrimSpace(*req.JourneyStage))).Error; err != nil {
				return err
			}
		}
		if req.Status != nil {
			if err := recordStatusHistory(tx, claims, "STEP", step.ID, previousStatus, *req.Status, req.Note); err != nil {
				return err
			}
		}
		if req.Status != nil && *req.Status == domain.StepStatusCompleted {
			if err := tx.Model(&domain.RenewalLink{}).Where("journey_step_id = ? AND status = ?", step.ID, domain.RenewalStatusSubmitted).
				Update("status", domain.RenewalStatusCompleted).Error; err != nil {
				return err
			}
			return advanceConfiguredSteps(tx, claims, &step)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Preload("AssignedMembership.User").First(&step, "id = ?", id).Error
	return &step, err
}

func (s *Service) RecordChurnEvent(ctx context.Context, claims *platformjwt.Claims, journeyID uuid.UUID, req domain.ChurnEventRequest) (*domain.PatientChurnEvent, error) {
	if err := journeyAllowed(s.db.WithContext(ctx), claims, journeyID); err != nil {
		return nil, err
	}
	var event domain.PatientChurnEvent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var journey domain.CareJourney
		if err := tx.First(&journey, "id = ? AND company_id = ?", journeyID, claims.CompanyID).Error; err != nil {
			return translate(err, "care journey")
		}
		event = domain.PatientChurnEvent{
			CompanyID: claims.CompanyID, JourneyID: journey.ID, PatientID: journey.PatientID,
			LocationID: journey.LocationID, PerformedByMembershipID: claims.MembershipID,
			EventType: req.EventType, StageAtEvent: journey.CurrentStage,
			ReasonCode: strings.ToUpper(strings.TrimSpace(req.ReasonCode)), Note: strings.TrimSpace(req.Note), OccurredAt: time.Now(),
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		return applyJourneyEvent(tx, &journey, req)
	})
	return &event, err
}

func createJourneyStep(tx *gorm.DB, companyID, journeyID uuid.UUID, req domain.JourneyStepRequest) (*domain.JourneyStep, error) {
	if req.AssignedMembershipID != nil {
		if err := validMembership(tx, companyID, *req.AssignedMembershipID); err != nil {
			return nil, err
		}
	}
	status := req.Status
	if status == "" {
		status = domain.StepStatusPending
	}
	cycle := req.CycleNumber
	if cycle == 0 {
		cycle = 1
	}
	metadata, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, httpx.BadRequest("invalid metadata")
	}
	if req.Metadata == nil {
		metadata = []byte("{}")
	}
	step := domain.JourneyStep{
		CompanyID: companyID, JourneyID: journeyID, AssignedMembershipID: req.AssignedMembershipID,
		StepKey: strings.ToLower(strings.TrimSpace(req.StepKey)), StepType: strings.ToUpper(strings.TrimSpace(req.StepType)),
		CycleNumber: cycle, Status: status, DueAt: req.DueAt, Metadata: datatypes.JSON(metadata),
	}
	if status == domain.StepStatusCompleted {
		now := time.Now()
		step.CompletedAt = &now
	}
	err = tx.Create(&step).Error
	return &step, translate(err, "journey step")
}

func nextJourneyEpisode(db *gorm.DB, companyID, patientID uuid.UUID, programCode string) (int, error) {
	var maxEpisode int
	row := db.Model(&domain.CareJourney{}).
		Where("company_id = ? AND patient_id = ? AND program_code = ?", companyID, patientID, programCode).
		Select("COALESCE(MAX(episode_number), 0)").Row()
	if err := row.Scan(&maxEpisode); err != nil {
		return 0, err
	}
	return maxEpisode + 1, nil
}

func journeyAllowed(db *gorm.DB, claims *platformjwt.Claims, journeyID uuid.UUID) error {
	query := db.Model(&domain.CareJourney{}).Where("care_journeys.id = ? AND care_journeys.company_id = ?", journeyID, claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		query = query.Where("EXISTS (SELECT 1 FROM location_assignments la WHERE la.membership_id = ? AND la.location_id = care_journeys.location_id)", claims.MembershipID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.NotFound("care journey")
	}
	return nil
}

func validMembership(db *gorm.DB, companyID, membershipID uuid.UUID) error {
	var count int64
	if err := db.Model(&domain.Membership{}).Where("id = ? AND company_id = ? AND status = ?", membershipID, companyID, domain.StatusActive).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.NotFound("active membership")
	}
	return nil
}

func applyJourneyEvent(tx *gorm.DB, journey *domain.CareJourney, req domain.ChurnEventRequest) error {
	now := time.Now()
	switch req.EventType {
	case "CHURNED":
		if err := tx.Model(journey).Updates(map[string]any{"status": domain.JourneyStatusChurned, "ended_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.JourneyStep{}).Where("journey_id = ? AND status NOT IN ?", journey.ID, []string{domain.StepStatusCompleted, domain.StepStatusCancelled, domain.StepStatusExpired}).Update("status", domain.StepStatusCancelled).Error; err != nil {
			return err
		}
		return revokeJourneyLinks(tx, journey.ID, now)
	case "REACTIVATED":
		stage := strings.ToUpper(strings.TrimSpace(req.NextStage))
		if stage == "" {
			stage = "RENEWAL_DUE"
		}
		return tx.Model(journey).Updates(map[string]any{"status": domain.JourneyStatusActive, "current_stage": stage, "ended_at": nil}).Error
	case "COMPLETED":
		return tx.Model(journey).Updates(map[string]any{"status": domain.JourneyStatusCompleted, "ended_at": now}).Error
	case "CLOSED":
		if err := tx.Model(journey).Updates(map[string]any{"status": domain.JourneyStatusCancelled, "ended_at": now}).Error; err != nil {
			return err
		}
		return revokeJourneyLinks(tx, journey.ID, now)
	case "TRANSFERRED":
		return nil
	default:
		return httpx.BadRequest("unsupported journey event")
	}
}

func revokeJourneyLinks(tx *gorm.DB, journeyID uuid.UUID, now time.Time) error {
	return tx.Model(&domain.RenewalLink{}).
		Where("journey_step_id IN (SELECT id FROM journey_steps WHERE journey_id = ?) AND status IN ?", journeyID, []string{domain.RenewalStatusIssued, domain.RenewalStatusOpened}).
		Updates(map[string]any{"status": domain.RenewalStatusRevoked, "revoked_at": now}).Error
}

func isTerminalJourneyStatus(status string) bool {
	return status == domain.JourneyStatusCompleted || status == domain.JourneyStatusChurned || status == domain.JourneyStatusCancelled
}
