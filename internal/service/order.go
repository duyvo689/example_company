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

func (s *Service) ListOrders(ctx context.Context, claims *platformjwt.Claims, page, pageSize int, status string, locationID, patientID, journeyStepID *uuid.UUID) (*domain.Page[domain.Order], error) {
	base := s.db.WithContext(ctx).Model(&domain.Order{}).Where("orders.company_id = ?", claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		base = base.Joins("JOIN location_assignments la ON la.location_id = orders.location_id AND la.membership_id = ?", claims.MembershipID)
	}
	if locationID != nil {
		if err := locationAllowed(s.db.WithContext(ctx), claims, *locationID); err != nil {
			return nil, err
		}
		base = base.Where("orders.location_id = ?", *locationID)
	}
	if patientID != nil {
		base = base.Where("orders.patient_id = ?", *patientID)
	}
	if journeyStepID != nil {
		base = base.Where("orders.journey_step_id = ?", *journeyStepID)
	}
	if status != "" {
		base = base.Where("orders.status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.Order
	if err := base.Preload("Location").Preload("Patient").Preload("JourneyStep").Order("orders.created_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetOrder(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID) (*domain.Order, error) {
	var order domain.Order
	query := s.db.WithContext(ctx).Preload("Location").Preload("Patient").Preload("CreatedBy.User").Preload("JourneyStep").Where("orders.id = ? AND orders.company_id = ?", id, claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		query = query.Joins("JOIN location_assignments la ON la.location_id = orders.location_id AND la.membership_id = ?", claims.MembershipID)
	}
	err := query.First(&order).Error
	return &order, translate(err, "order")
}

func (s *Service) CreateOrder(ctx context.Context, claims *platformjwt.Claims, req domain.OrderRequest) (*domain.Order, error) {
	if err := locationAllowed(s.db.WithContext(ctx), claims, req.LocationID); err != nil {
		return nil, err
	}
	if err := s.patientAllowed(s.db.WithContext(ctx), claims, req.PatientID); err != nil {
		return nil, err
	}
	if err := patientAtLocation(s.db.WithContext(ctx), req.PatientID, req.LocationID); err != nil {
		return nil, err
	}
	if req.JourneyStepID != nil {
		if err := validOrderStep(s.db.WithContext(ctx), claims, *req.JourneyStepID, req.PatientID, req.LocationID); err != nil {
			return nil, err
		}
	}
	status := req.Status
	if status == "" {
		status = domain.StatusPending
	}
	order := domain.Order{CompanyID: claims.CompanyID, LocationID: req.LocationID, PatientID: req.PatientID, CreatedByMembershipID: claims.MembershipID, JourneyStepID: req.JourneyStepID, ExternalOrderID: strings.TrimSpace(req.ExternalOrderID), Status: status}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return translate(err, "order")
		}
		return createOrderEvent(tx, order, "order.created")
	})
	return &order, err
}

func (s *Service) UpdateOrder(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID, req domain.OrderUpdateRequest) (*domain.Order, error) {
	current, err := s.GetOrder(ctx, claims, id)
	if err != nil {
		return nil, err
	}
	if req.LocationID == nil && req.PatientID == nil && req.JourneyStepID == nil && req.ExternalOrderID == nil && req.Status == nil {
		return current, nil
	}
	if req.LocationID != nil {
		if err := locationAllowed(s.db.WithContext(ctx), claims, *req.LocationID); err != nil {
			return nil, err
		}
		current.LocationID = *req.LocationID
	}
	if req.PatientID != nil {
		if err := s.patientAllowed(s.db.WithContext(ctx), claims, *req.PatientID); err != nil {
			return nil, err
		}
		current.PatientID = *req.PatientID
	}
	if req.JourneyStepID != nil {
		current.JourneyStepID = req.JourneyStepID
	}
	if req.ExternalOrderID != nil {
		current.ExternalOrderID = strings.TrimSpace(*req.ExternalOrderID)
	}
	if req.Status != nil {
		current.Status = *req.Status
	}
	if err := patientAtLocation(s.db.WithContext(ctx), current.PatientID, current.LocationID); err != nil {
		return nil, err
	}
	if current.JourneyStepID != nil {
		if err := validOrderStep(s.db.WithContext(ctx), claims, *current.JourneyStepID, current.PatientID, current.LocationID); err != nil {
			return nil, err
		}
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(current).Error; err != nil {
			return translate(err, "order")
		}
		return createOrderEvent(tx, *current, "order.updated")
	})
	return current, err
}

func validOrderStep(db *gorm.DB, claims *platformjwt.Claims, stepID, patientID, locationID uuid.UUID) error {
	var step domain.JourneyStep
	if err := db.Preload("Journey").First(&step, "id = ? AND company_id = ?", stepID, claims.CompanyID).Error; err != nil {
		return translate(err, "journey step")
	}
	if err := journeyAllowed(db, claims, step.JourneyID); err != nil {
		return err
	}
	if step.Journey == nil || step.Journey.PatientID != patientID || step.Journey.LocationID != locationID {
		return httpx.BadRequest("journey step, patient and location do not match")
	}
	if isTerminalJourneyStatus(step.Journey.Status) {
		return httpx.Conflict("cannot create an order for a terminal journey")
	}
	return nil
}

func patientAtLocation(db *gorm.DB, patientID, locationID uuid.UUID) error {
	var count int64
	err := db.Model(&domain.Patient{}).
		Where("id = ? AND (primary_location_id = ? OR EXISTS (SELECT 1 FROM patient_locations pl WHERE pl.patient_id = patients.id AND pl.location_id = ?))", patientID, locationID, locationID).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return httpx.BadRequest("patient is not linked to the order location")
	}
	return nil
}

func (s *Service) DeleteOrder(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID) error {
	order, err := s.GetOrder(ctx, claims, id)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(order).Error; err != nil {
			return err
		}
		return createOrderEvent(tx, *order, "order.deleted")
	})
}

func createOrderEvent(tx *gorm.DB, order domain.Order, eventType string) error {
	payload, err := json.Marshal(order)
	if err != nil {
		return err
	}
	event := domain.IntegrationEvent{CompanyID: order.CompanyID, AggregateType: "order", AggregateID: order.ID, EventType: eventType, Payload: datatypes.JSON(payload), Status: domain.StatusPending, NextAttemptAt: time.Now()}
	return tx.Create(&event).Error
}
