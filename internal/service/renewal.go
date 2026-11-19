package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"example-company/internal/platform/password"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) IssueRenewalLink(ctx context.Context, claims *platformjwt.Claims, stepID uuid.UUID, req domain.IssueRenewalLinkRequest) (*domain.IssueRenewalLinkResponse, error) {
	var step domain.JourneyStep
	if err := s.db.WithContext(ctx).Preload("Journey").First(&step, "id = ? AND company_id = ?", stepID, claims.CompanyID).Error; err != nil {
		return nil, translate(err, "journey step")
	}
	if err := journeyAllowed(s.db.WithContext(ctx), claims, step.JourneyID); err != nil {
		return nil, err
	}
	if step.Journey == nil || isTerminalJourneyStatus(step.Journey.Status) {
		return nil, httpx.Conflict("cannot issue a renewal link for a terminal journey")
	}
	if step.Status == domain.StepStatusCompleted || step.Status == domain.StepStatusCancelled || step.Status == domain.StepStatusExpired {
		return nil, httpx.Conflict("cannot issue a renewal link for a closed step")
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	verificationHash, err := password.Hash(normalizeVerification(req.VerificationValue))
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(14 * 24 * time.Hour)
	if req.ExpiresAt != nil {
		expiresAt = *req.ExpiresAt
	} else if req.ExpiresInDays > 0 {
		expiresAt = time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
	}
	if !expiresAt.After(time.Now()) {
		return nil, httpx.BadRequest("expires_at must be in the future")
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 5
	}
	link := domain.RenewalLink{
		CompanyID: claims.CompanyID, JourneyStepID: step.ID, TokenHash: platformjwt.Hash(token),
		VerificationHash: verificationHash, Status: domain.RenewalStatusIssued,
		MaxAttempts: maxAttempts, ExpiresAt: expiresAt,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&domain.RenewalLink{}).Where("journey_step_id = ? AND status IN ?", step.ID, []string{domain.RenewalStatusIssued, domain.RenewalStatusOpened}).
			Updates(map[string]any{"status": domain.RenewalStatusRevoked, "revoked_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
		if err := tx.Model(&step).Updates(map[string]any{"status": domain.StepStatusWaitingPatient}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.CareJourney{}).Where("id = ?", step.JourneyID).Update("current_stage", "RENEWAL_DUE").Error
	})
	if err != nil {
		return nil, err
	}
	return &domain.IssueRenewalLinkResponse{RenewalLink: link, Token: token}, nil
}

func (s *Service) ListRenewalLinks(ctx context.Context, claims *platformjwt.Claims, stepID uuid.UUID) ([]domain.RenewalLink, error) {
	var step domain.JourneyStep
	if err := s.db.WithContext(ctx).First(&step, "id = ? AND company_id = ?", stepID, claims.CompanyID).Error; err != nil {
		return nil, translate(err, "journey step")
	}
	if err := journeyAllowed(s.db.WithContext(ctx), claims, step.JourneyID); err != nil {
		return nil, err
	}
	var links []domain.RenewalLink
	err := s.db.WithContext(ctx).Where("journey_step_id = ?", stepID).Order("created_at DESC").Find(&links).Error
	return links, err
}

func (s *Service) OpenRenewalLink(ctx context.Context, token, verification string) (*domain.RenewalLinkPublicResponse, error) {
	link, step, err := s.verifyRenewal(ctx, token, verification, true)
	if err != nil {
		return nil, err
	}
	intakeType := renewalIntakeType(step.Journey.ProgramCode)
	assignment, err := patientFormForStep(s.db.WithContext(ctx), step)
	if err != nil {
		return nil, err
	}
	var formVersionID *uuid.UUID
	var formSchema map[string]any
	if assignment != nil && assignment.FormVersion != nil {
		id := assignment.FormVersionID
		formVersionID = &id
		_ = json.Unmarshal(assignment.FormVersion.Schema, &formSchema)
	}
	return &domain.RenewalLinkPublicResponse{
		Status: link.Status, ExpiresAt: link.ExpiresAt, JourneyStepID: link.JourneyStepID,
		StepType: step.StepType, IntakeType: intakeType, FormVersionID: formVersionID, FormSchema: formSchema,
	}, nil
}

func (s *Service) SubmitRenewal(ctx context.Context, token string, req domain.SubmitRenewalRequest) (*domain.IntakeSubmission, bool, error) {
	link, step, err := s.verifyRenewal(ctx, token, req.VerificationValue, false)
	if err != nil {
		return nil, false, err
	}
	assignment, err := patientFormForStep(s.db.WithContext(ctx), step)
	if err != nil {
		return nil, false, err
	}
	var formVersionID *uuid.UUID
	schemaVersion := req.SchemaVersion
	if assignment != nil && assignment.FormVersion != nil {
		if err := validateFormAnswers(assignment.FormVersion.Schema, req.Answers); err != nil {
			return nil, false, err
		}
		id := assignment.FormVersionID
		formVersionID = &id
		schemaVersion = assignment.FormVersion.VersionNumber
	}
	answers, err := json.Marshal(req.Answers)
	if err != nil {
		return nil, false, httpx.BadRequest("invalid answers")
	}
	flags, err := json.Marshal(req.ClinicalFlags)
	if err != nil {
		return nil, false, httpx.BadRequest("invalid clinical_flags")
	}
	if req.ClinicalFlags == nil {
		flags = []byte("{}")
	}
	var submission domain.IntakeSubmission
	created := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(link, "id = ?", link.ID).Error; err != nil {
			return err
		}
		findErr := tx.Where("renewal_link_id = ?", link.ID).First(&submission).Error
		if findErr == nil {
			return nil
		}
		if findErr != gorm.ErrRecordNotFound {
			return findErr
		}
		if link.Status != domain.RenewalStatusIssued && link.Status != domain.RenewalStatusOpened {
			return httpx.Conflict("renewal link has already been used")
		}
		journey := step.Journey
		if journey == nil {
			return httpx.NotFound("care journey")
		}
		now := time.Now()
		submission = domain.IntakeSubmission{
			CompanyID: journey.CompanyID, FormVersionID: formVersionID, PatientID: &journey.PatientID, LocationID: journey.LocationID,
			JourneyID: &journey.ID, JourneyStepID: &step.ID, RenewalLinkID: &link.ID,
			IntakeType: req.IntakeType, SchemaVersion: schemaVersion, Status: "SUBMITTED",
			Answers: datatypes.JSON(answers), ClinicalFlags: datatypes.JSON(flags), SubmittedAt: now,
		}
		if err := tx.Create(&submission).Error; err != nil {
			return translate(err, "intake submission")
		}
		created = true
		if err := tx.Model(link).Updates(map[string]any{"status": domain.RenewalStatusSubmitted, "submitted_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(step).Update("status", domain.StepStatusInReview).Error; err != nil {
			return err
		}
		return tx.Model(&domain.CareJourney{}).Where("id = ?", journey.ID).Update("current_stage", "PROVIDER_REVIEW").Error
	})
	return &submission, created, err
}

func patientFormForStep(db *gorm.DB, step *domain.JourneyStep) (*domain.StepFormAssignment, error) {
	if step.StepTemplateID == nil {
		return nil, nil
	}
	var assignment domain.StepFormAssignment
	err := db.Preload("FormVersion").
		Where("step_template_id = ? AND actor_type = ? AND phase IN ?", *step.StepTemplateID, "PATIENT", []string{"BEFORE", "DURING"}).
		Order("is_required DESC, created_at ASC").First(&assignment).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (s *Service) verifyRenewal(ctx context.Context, token, verification string, markOpened bool) (*domain.RenewalLink, *domain.JourneyStep, error) {
	var link domain.RenewalLink
	err := s.db.WithContext(ctx).Where("token_hash = ?", platformjwt.Hash(token)).First(&link).Error
	if err != nil {
		return nil, nil, httpx.NotFound("renewal link")
	}
	if link.ExpiresAt.Before(time.Now()) && link.Status != domain.RenewalStatusCompleted && link.Status != domain.RenewalStatusSubmitted {
		_ = s.db.WithContext(ctx).Model(&link).Update("status", domain.RenewalStatusExpired).Error
		return nil, nil, httpx.Conflict("renewal link has expired")
	}
	if link.Status == domain.RenewalStatusRevoked || link.Status == domain.RenewalStatusExpired || link.Status == domain.RenewalStatusLocked {
		return nil, nil, httpx.Conflict("renewal link is not active")
	}
	if markOpened && link.Status != domain.RenewalStatusIssued && link.Status != domain.RenewalStatusOpened {
		return nil, nil, httpx.Conflict("renewal link has already been used")
	}
	if !password.Verify(link.VerificationHash, normalizeVerification(verification)) {
		attempts := link.AttemptCount + 1
		updates := map[string]any{"attempt_count": attempts}
		if attempts >= link.MaxAttempts {
			updates["status"] = domain.RenewalStatusLocked
		}
		_ = s.db.WithContext(ctx).Model(&link).Updates(updates).Error
		return nil, nil, httpx.Unauthorized("renewal verification failed")
	}
	var step domain.JourneyStep
	if err := s.db.WithContext(ctx).Preload("Journey").First(&step, "id = ?", link.JourneyStepID).Error; err != nil {
		return nil, nil, translate(err, "journey step")
	}
	if markOpened && link.Status == domain.RenewalStatusIssued {
		now := time.Now()
		if err := s.db.WithContext(ctx).Model(&link).Updates(map[string]any{"status": domain.RenewalStatusOpened, "opened_at": now}).Error; err != nil {
			return nil, nil, err
		}
		link.Status = domain.RenewalStatusOpened
		link.OpenedAt = &now
	}
	return &link, &step, nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func normalizeVerification(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func renewalIntakeType(programCode string) string {
	if strings.Contains(strings.ToUpper(programCode), "GLP") {
		return "GLP1_RENEWAL"
	}
	return "TRT_RENEWAL"
}
