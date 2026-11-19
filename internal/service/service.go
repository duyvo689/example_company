package service

import (
	"errors"
	"strings"

	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	tokens *platformjwt.Manager
}

func New(db *gorm.DB, tokens *platformjwt.Manager) *Service {
	return &Service{db: db, tokens: tokens}
}

func translate(err error, resource string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return httpx.NotFound(resource)
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "duplicate key") || strings.Contains(message, "unique constraint") {
		return httpx.Conflict(resource + " already exists")
	}
	if strings.Contains(message, "foreign key constraint") {
		return httpx.BadRequest("referenced resource is invalid")
	}
	return err
}

func pageOffset(page, pageSize int) int { return (page - 1) * pageSize }

func validLocation(db *gorm.DB, companyID, locationID uuid.UUID) error {
	var count int64
	if err := db.Model(&struct{ ID uuid.UUID }{}).Table("locations").Where("id = ? AND company_id = ? AND deleted_at IS NULL", locationID, companyID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.NotFound("location")
	}
	return nil
}

func locationAllowed(db *gorm.DB, claims *platformjwt.Claims, locationID uuid.UUID) error {
	if err := validLocation(db, claims.CompanyID, locationID); err != nil {
		return err
	}
	if claims.Scope == "company" {
		return nil
	}
	var count int64
	if err := db.Table("location_assignments").Where("membership_id = ? AND location_id = ?", claims.MembershipID, locationID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httpx.Forbidden("location is outside your assignment")
	}
	return nil
}
