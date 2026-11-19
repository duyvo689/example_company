package service

import (
	"context"
	"encoding/json"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) ListIntakeSubmissions(ctx context.Context, claims *platformjwt.Claims, page, pageSize int, intakeType, status string, journeyID, patientID *uuid.UUID) (*domain.Page[domain.IntakeSubmission], error) {
	base := s.db.WithContext(ctx).Model(&domain.IntakeSubmission{}).Where("intake_submissions.company_id = ?", claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		base = base.Where("EXISTS (SELECT 1 FROM location_assignments la WHERE la.membership_id = ? AND la.location_id = intake_submissions.location_id)", claims.MembershipID)
	}
	if intakeType != "" {
		base = base.Where("intake_submissions.intake_type = ?", intakeType)
	}
	if status != "" {
		base = base.Where("intake_submissions.status = ?", status)
	}
	if journeyID != nil {
		base = base.Where("intake_submissions.journey_id = ?", *journeyID)
	}
	if patientID != nil {
		base = base.Where("intake_submissions.patient_id = ?", *patientID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.IntakeSubmission
	if err := base.Order("intake_submissions.submitted_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetIntakeSubmission(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID) (*domain.IntakeSubmission, error) {
	query := s.db.WithContext(ctx).Preload("Patient").Preload("Location").Preload("Journey").Preload("JourneyStep").Preload("FormVersion.FormTemplate").Preload("Files").
		Where("intake_submissions.id = ? AND intake_submissions.company_id = ?", id, claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		query = query.Where("EXISTS (SELECT 1 FROM location_assignments la WHERE la.membership_id = ? AND la.location_id = intake_submissions.location_id)", claims.MembershipID)
	}
	var submission domain.IntakeSubmission
	err := query.First(&submission).Error
	return &submission, translate(err, "intake submission")
}

func (s *Service) CreateIntakeSubmission(ctx context.Context, claims *platformjwt.Claims, req domain.CreateIntakeSubmissionRequest) (*domain.IntakeSubmission, error) {
	if err := locationAllowed(s.db.WithContext(ctx), claims, req.LocationID); err != nil {
		return nil, err
	}
	if req.PatientID != nil {
		if err := s.patientAllowed(s.db.WithContext(ctx), claims, *req.PatientID); err != nil {
			return nil, err
		}
		if err := patientAtLocation(s.db.WithContext(ctx), *req.PatientID, req.LocationID); err != nil {
			return nil, err
		}
	}
	if err := validateIntakeLinks(s.db.WithContext(ctx), claims, req); err != nil {
		return nil, err
	}
	if req.FormVersionID != nil {
		var formVersion domain.FormVersion
		if err := s.db.WithContext(ctx).Preload("FormTemplate").First(&formVersion, "id = ? AND status <> ?", *req.FormVersionID, domain.ConfigStatusDraft).Error; err != nil {
			return nil, httpx.NotFound("usable form version")
		}
		if formVersion.FormTemplate == nil || formVersion.FormTemplate.CompanyID != claims.CompanyID {
			return nil, httpx.NotFound("usable form version")
		}
		if err := validateFormAnswers(formVersion.Schema, req.Answers); err != nil {
			return nil, err
		}
		if req.JourneyStepID != nil {
			var count int64
			if err := s.db.WithContext(ctx).Model(&domain.StepFormAssignment{}).Where("step_template_id = (SELECT step_template_id FROM journey_steps WHERE id = ?) AND form_version_id = ?", *req.JourneyStepID, *req.FormVersionID).Count(&count).Error; err != nil {
				return nil, err
			}
			if count == 0 {
				return nil, httpx.BadRequest("form version is not assigned to this journey step")
			}
		}
	}
	answers, err := json.Marshal(req.Answers)
	if err != nil {
		return nil, httpx.BadRequest("invalid answers")
	}
	flags, err := json.Marshal(req.ClinicalFlags)
	if err != nil {
		return nil, httpx.BadRequest("invalid clinical_flags")
	}
	if req.ClinicalFlags == nil {
		flags = []byte("{}")
	}
	status := req.Status
	if status == "" {
		status = "SUBMITTED"
	}
	submission := domain.IntakeSubmission{
		CompanyID: claims.CompanyID, FormVersionID: req.FormVersionID, PatientID: req.PatientID, LocationID: req.LocationID,
		JourneyID: req.JourneyID, JourneyStepID: req.JourneyStepID, IntakeType: req.IntakeType,
		SchemaVersion: req.SchemaVersion, Status: status, Answers: datatypes.JSON(answers),
		ClinicalFlags: datatypes.JSON(flags), SubmittedAt: time.Now(),
	}
	err = s.db.WithContext(ctx).Create(&submission).Error
	return &submission, translate(err, "intake submission")
}

func (s *Service) AddIntakeFile(ctx context.Context, claims *platformjwt.Claims, submissionID uuid.UUID, req domain.CreateIntakeFileRequest) (*domain.IntakeFile, error) {
	if _, err := s.GetIntakeSubmission(ctx, claims, submissionID); err != nil {
		return nil, err
	}
	file := domain.IntakeFile{
		CompanyID: claims.CompanyID, SubmissionID: submissionID, FileType: req.FileType,
		StorageKey: req.StorageKey, MimeType: req.MimeType,
	}
	err := s.db.WithContext(ctx).Create(&file).Error
	return &file, translate(err, "intake file")
}

func validateIntakeLinks(db *gorm.DB, claims *platformjwt.Claims, req domain.CreateIntakeSubmissionRequest) error {
	if req.JourneyStepID != nil && req.JourneyID == nil {
		return httpx.BadRequest("journey_id is required when journey_step_id is provided")
	}
	if req.JourneyID == nil {
		return nil
	}
	if err := journeyAllowed(db, claims, *req.JourneyID); err != nil {
		return err
	}
	var journey domain.CareJourney
	if err := db.First(&journey, "id = ?", *req.JourneyID).Error; err != nil {
		return translate(err, "care journey")
	}
	if journey.LocationID != req.LocationID {
		return httpx.BadRequest("journey and intake location do not match")
	}
	if req.PatientID != nil && journey.PatientID != *req.PatientID {
		return httpx.BadRequest("journey and intake patient do not match")
	}
	if req.JourneyStepID != nil {
		var count int64
		if err := db.Model(&domain.JourneyStep{}).Where("id = ? AND journey_id = ? AND company_id = ?", *req.JourneyStepID, journey.ID, claims.CompanyID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return httpx.NotFound("journey step")
		}
	}
	return nil
}
