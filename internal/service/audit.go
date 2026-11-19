package service

import (
	"context"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	"github.com/google/uuid"
)

func (s *Service) ListAuditLogs(ctx context.Context, companyID uuid.UUID, page, pageSize int, action, membershipID, from, to string) (*domain.Page[domain.AuditLog], error) {
	base := s.db.WithContext(ctx).Model(&domain.AuditLog{}).Where("company_id = ?", companyID)
	if action != "" {
		base = base.Where("action = ?", action)
	}
	if membershipID != "" {
		id, err := uuid.Parse(membershipID)
		if err != nil {
			return nil, httpx.BadRequest("invalid membership_id")
		}
		base = base.Where("membership_id = ?", id)
	}
	if from != "" {
		value, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return nil, httpx.BadRequest("from must be RFC3339")
		}
		base = base.Where("created_at >= ?", value)
	}
	if to != "" {
		value, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return nil, httpx.BadRequest("to must be RFC3339")
		}
		base = base.Where("created_at <= ?", value)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.AuditLog
	if err := base.Order("created_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetAuditLog(ctx context.Context, companyID, id uuid.UUID) (*domain.AuditLog, error) {
	var item domain.AuditLog
	err := s.db.WithContext(ctx).First(&item, "id = ? AND company_id = ?", id, companyID).Error
	return &item, translate(err, "audit log")
}

func (s *Service) ListIntegrationEvents(ctx context.Context, companyID uuid.UUID, page, pageSize int, status string) (*domain.Page[domain.IntegrationEvent], error) {
	base := s.db.WithContext(ctx).Model(&domain.IntegrationEvent{}).Where("company_id = ?", companyID)
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.IntegrationEvent
	if err := base.Order("created_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) RetryIntegrationEvent(ctx context.Context, companyID, id uuid.UUID) (*domain.IntegrationEvent, error) {
	updates := map[string]any{"status": domain.StatusPending, "next_attempt_at": time.Now(), "last_error": ""}
	result := s.db.WithContext(ctx).Model(&domain.IntegrationEvent{}).Where("id = ? AND company_id = ?", id, companyID).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, httpx.NotFound("integration event")
	}
	var event domain.IntegrationEvent
	err := s.db.WithContext(ctx).First(&event, "id = ?", id).Error
	return &event, err
}
