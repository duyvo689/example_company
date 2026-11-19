package service

import (
	"context"
	"strings"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) ListPatients(ctx context.Context, claims *platformjwt.Claims, page, pageSize int, search, status string, locationID *uuid.UUID) (*domain.Page[domain.Patient], error) {
	base := s.db.WithContext(ctx).Model(&domain.Patient{}).Where("patients.company_id = ?", claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		base = base.Where(`EXISTS (
			SELECT 1 FROM location_assignments la
			LEFT JOIN patient_locations pl ON pl.location_id = la.location_id AND pl.patient_id = patients.id
			WHERE la.membership_id = ? AND (pl.patient_id IS NOT NULL OR patients.primary_location_id = la.location_id)
		)`, claims.MembershipID)
	}
	if locationID != nil {
		if err := locationAllowed(s.db.WithContext(ctx), claims, *locationID); err != nil {
			return nil, err
		}
		base = base.Where("patients.primary_location_id = ? OR EXISTS (SELECT 1 FROM patient_locations pl WHERE pl.patient_id = patients.id AND pl.location_id = ?)", *locationID, *locationID)
	}
	if search != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		base = base.Where("LOWER(patients.public_ref) LIKE ? OR LOWER(patients.external_chart_id) LIKE ?", like, like)
	}
	if status != "" {
		base = base.Where("patients.status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.Patient
	if err := base.Preload("PrimaryLocation").Order("patients.created_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetPatient(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID) (*domain.PatientDetail, error) {
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, id); err != nil {
		return nil, err
	}
	var patient domain.Patient
	if err := s.db.WithContext(ctx).Preload("PrimaryLocation").First(&patient, "id = ? AND company_id = ?", id, claims.CompanyID).Error; err != nil {
		return nil, translate(err, "patient")
	}
	var locations []domain.PatientLocation
	if err := s.db.WithContext(ctx).Preload("Location").Where("patient_id = ?", id).Find(&locations).Error; err != nil {
		return nil, err
	}
	return &domain.PatientDetail{Patient: patient, Locations: locations}, nil
}

func (s *Service) CreatePatient(ctx context.Context, claims *platformjwt.Claims, req domain.PatientRequest) (*domain.Patient, error) {
	if req.PrimaryLocationID != nil {
		if err := locationAllowed(s.db.WithContext(ctx), claims, *req.PrimaryLocationID); err != nil {
			return nil, err
		}
	} else if claims.Scope == domain.ScopeLocation {
		return nil, httpx.BadRequest("primary_location_id is required for a location-scoped membership")
	}
	status := req.Status
	if status == "" {
		status = domain.StatusActive
	}
	patient := domain.Patient{CompanyID: claims.CompanyID, PrimaryLocationID: req.PrimaryLocationID, PublicRef: strings.TrimSpace(req.PublicRef), ExternalChartID: strings.TrimSpace(req.ExternalChartID), Status: status}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&patient).Error; err != nil {
			return translate(err, "patient")
		}
		if patient.PrimaryLocationID != nil {
			link := domain.PatientLocation{PatientID: patient.ID, LocationID: *patient.PrimaryLocationID, Relationship: "primary"}
			if err := tx.Create(&link).Error; err != nil {
				return translate(err, "patient location")
			}
		}
		return nil
	})
	return &patient, err
}

func (s *Service) UpdatePatient(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID, req domain.PatientUpdateRequest) (*domain.Patient, error) {
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, id); err != nil {
		return nil, err
	}
	if req.PrimaryLocationID != nil {
		if err := locationAllowed(s.db.WithContext(ctx), claims, *req.PrimaryLocationID); err != nil {
			return nil, err
		}
	}
	updates := map[string]any{}
	if req.PrimaryLocationID != nil {
		updates["primary_location_id"] = *req.PrimaryLocationID
	}
	if req.PublicRef != nil {
		updates["public_ref"] = strings.TrimSpace(*req.PublicRef)
	}
	if req.ExternalChartID != nil {
		updates["external_chart_id"] = strings.TrimSpace(*req.ExternalChartID)
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if len(updates) == 0 {
		detail, err := s.GetPatient(ctx, claims, id)
		if err != nil {
			return nil, err
		}
		return &detail.Patient, nil
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&domain.Patient{}).Where("id = ? AND company_id = ?", id, claims.CompanyID).Updates(updates)
		if result.Error != nil {
			return translate(result.Error, "patient")
		}
		if result.RowsAffected == 0 {
			return httpx.NotFound("patient")
		}
		if req.PrimaryLocationID != nil {
			if err := tx.Model(&domain.PatientLocation{}).
				Where("patient_id = ? AND location_id <> ? AND relationship = ?", id, *req.PrimaryLocationID, "primary").
				Update("relationship", "secondary").Error; err != nil {
				return err
			}
			link := domain.PatientLocation{PatientID: id, LocationID: *req.PrimaryLocationID, Relationship: "primary"}
			return tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "patient_id"}, {Name: "location_id"}},
				DoUpdates: clause.Assignments(map[string]any{"relationship": "primary"}),
			}).Create(&link).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var patient domain.Patient
	err = s.db.WithContext(ctx).Preload("PrimaryLocation").First(&patient, "id = ?", id).Error
	return &patient, err
}

func (s *Service) DeletePatient(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID) error {
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, id); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&domain.Order{}).Where("patient_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return httpx.Conflict("patient is referenced by orders; set it inactive instead")
		}
		if err := tx.Where("patient_id = ?", id).Delete(&domain.PatientLocation{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND company_id = ?", id, claims.CompanyID).Delete(&domain.Patient{})
		if result.RowsAffected == 0 {
			return httpx.NotFound("patient")
		}
		return result.Error
	})
}

func (s *Service) AddPatientLocation(ctx context.Context, claims *platformjwt.Claims, patientID uuid.UUID, req domain.PatientLocationRequest) (*domain.PatientLocation, error) {
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, patientID); err != nil {
		return nil, err
	}
	if err := locationAllowed(s.db.WithContext(ctx), claims, req.LocationID); err != nil {
		return nil, err
	}
	link := domain.PatientLocation{PatientID: patientID, LocationID: req.LocationID, Relationship: strings.TrimSpace(req.Relationship)}
	err := s.db.WithContext(ctx).Create(&link).Error
	return &link, translate(err, "patient location")
}

func (s *Service) DeletePatientLocation(ctx context.Context, claims *platformjwt.Claims, patientID, locationID uuid.UUID) error {
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, patientID); err != nil {
		return err
	}
	if err := locationAllowed(s.db.WithContext(ctx), claims, locationID); err != nil {
		return err
	}
	var patient domain.Patient
	if err := s.db.WithContext(ctx).First(&patient, "id = ?", patientID).Error; err != nil {
		return translate(err, "patient")
	}
	if patient.PrimaryLocationID != nil && *patient.PrimaryLocationID == locationID {
		return httpx.Conflict("primary location cannot be removed")
	}
	result := s.db.WithContext(ctx).Where("patient_id = ? AND location_id = ?", patientID, locationID).Delete(&domain.PatientLocation{})
	if result.RowsAffected == 0 {
		return httpx.NotFound("patient location")
	}
	return result.Error
}

func (s *Service) patientAllowed(db *gorm.DB, claims *platformjwt.Claims, patientID uuid.UUID) error {
	query := db.Model(&domain.Patient{}).Where("patients.id = ? AND patients.company_id = ?", patientID, claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		query = query.Where(`EXISTS (
			SELECT 1 FROM location_assignments la
			LEFT JOIN patient_locations pl ON pl.location_id = la.location_id AND pl.patient_id = patients.id
			WHERE la.membership_id = ? AND (pl.patient_id IS NOT NULL OR patients.primary_location_id = la.location_id)
		)`, claims.MembershipID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.NotFound("patient")
	}
	return nil
}
