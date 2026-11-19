package service

import (
	"context"
	"strings"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) GetCompany(ctx context.Context, companyID uuid.UUID) (*domain.Company, error) {
	var company domain.Company
	err := s.db.WithContext(ctx).First(&company, "id = ?", companyID).Error
	return &company, translate(err, "company")
}

func (s *Service) UpdateCompany(ctx context.Context, companyID uuid.UUID, req domain.UpdateCompanyRequest) (*domain.Company, error) {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if len(updates) > 0 {
		result := s.db.WithContext(ctx).Model(&domain.Company{}).Where("id = ?", companyID).Updates(updates)
		if result.Error != nil {
			return nil, translate(result.Error, "company")
		}
		if result.RowsAffected == 0 {
			return nil, httpx.NotFound("company")
		}
	}
	return s.GetCompany(ctx, companyID)
}

func (s *Service) ListLocations(ctx context.Context, claims *platformjwt.Claims, page, pageSize int, search, status string) (*domain.Page[domain.Location], error) {
	base := s.db.WithContext(ctx).Model(&domain.Location{}).Where("locations.company_id = ?", claims.CompanyID)
	if claims.Scope == domain.ScopeLocation {
		base = base.Joins("JOIN location_assignments la ON la.location_id = locations.id AND la.membership_id = ?", claims.MembershipID)
	}
	if search != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		base = base.Where("LOWER(locations.name) LIKE ? OR LOWER(locations.code) LIKE ?", like, like)
	}
	if status != "" {
		base = base.Where("locations.status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.Location
	if err := base.Order("locations.created_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetLocation(ctx context.Context, claims *platformjwt.Claims, id uuid.UUID) (*domain.Location, error) {
	if err := locationAllowed(s.db.WithContext(ctx), claims, id); err != nil {
		return nil, err
	}
	var location domain.Location
	err := s.db.WithContext(ctx).First(&location, "id = ? AND company_id = ?", id, claims.CompanyID).Error
	return &location, translate(err, "location")
}

func (s *Service) CreateLocation(ctx context.Context, companyID uuid.UUID, req domain.LocationRequest) (*domain.Location, error) {
	status := req.Status
	if status == "" {
		status = domain.StatusActive
	}
	location := domain.Location{CompanyID: companyID, Code: strings.ToUpper(strings.TrimSpace(req.Code)), Name: strings.TrimSpace(req.Name), Status: status}
	err := s.db.WithContext(ctx).Create(&location).Error
	return &location, translate(err, "location")
}

func (s *Service) UpdateLocation(ctx context.Context, companyID, id uuid.UUID, req domain.LocationUpdateRequest) (*domain.Location, error) {
	updates := map[string]any{}
	if req.Code != nil {
		updates["code"] = strings.ToUpper(strings.TrimSpace(*req.Code))
	}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if len(updates) == 0 {
		var location domain.Location
		err := s.db.WithContext(ctx).First(&location, "id = ? AND company_id = ?", id, companyID).Error
		return &location, translate(err, "location")
	}
	result := s.db.WithContext(ctx).Model(&domain.Location{}).Where("id = ? AND company_id = ?", id, companyID).Updates(updates)
	if result.Error != nil {
		return nil, translate(result.Error, "location")
	}
	if result.RowsAffected == 0 {
		return nil, httpx.NotFound("location")
	}
	var location domain.Location
	err := s.db.WithContext(ctx).First(&location, "id = ?", id).Error
	return &location, err
}

func (s *Service) DeleteLocation(ctx context.Context, companyID, id uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&domain.Order{}).Where("location_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return httpx.Conflict("location is referenced by orders; set it inactive instead")
		}
		result := tx.Where("id = ? AND company_id = ?", id, companyID).Delete(&domain.Location{})
		if result.Error != nil {
			return translate(result.Error, "location")
		}
		if result.RowsAffected == 0 {
			return httpx.NotFound("location")
		}
		return nil
	})
}

func (s *Service) ListRoles(ctx context.Context, page, pageSize int) (*domain.Page[domain.Role], error) {
	var total int64
	base := s.db.WithContext(ctx).Model(&domain.Role{})
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.Role
	if err := base.Order("code ASC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) CreateRole(ctx context.Context, req domain.RoleRequest) (*domain.Role, error) {
	role := domain.Role{Code: strings.ToUpper(strings.TrimSpace(req.Code)), Scope: req.Scope, Description: strings.TrimSpace(req.Description)}
	err := s.db.WithContext(ctx).Create(&role).Error
	return &role, translate(err, "role")
}

func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, req domain.RoleRequest) (*domain.Role, error) {
	var existing domain.Role
	if err := s.db.WithContext(ctx).First(&existing, "id = ?", id).Error; err != nil {
		return nil, translate(err, "role")
	}
	if existing.Code == "ADMIN" || existing.Code == "STAFF" || existing.Code == "CLINICIAN" {
		return nil, httpx.Conflict("built-in roles cannot be modified")
	}
	updates := map[string]any{"code": strings.ToUpper(strings.TrimSpace(req.Code)), "scope": req.Scope, "description": strings.TrimSpace(req.Description)}
	result := s.db.WithContext(ctx).Model(&domain.Role{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, translate(result.Error, "role")
	}
	if result.RowsAffected == 0 {
		return nil, httpx.NotFound("role")
	}
	var role domain.Role
	err := s.db.WithContext(ctx).First(&role, "id = ?", id).Error
	return &role, err
}

func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) error {
	var role domain.Role
	if err := s.db.WithContext(ctx).First(&role, "id = ?", id).Error; err != nil {
		return translate(err, "role")
	}
	if role.Code == "ADMIN" || role.Code == "STAFF" || role.Code == "CLINICIAN" {
		return httpx.Conflict("built-in roles cannot be deleted")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&domain.Membership{}).Where("role_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return httpx.Conflict("role is assigned to memberships")
	}
	result := s.db.WithContext(ctx).Delete(&domain.Role{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return httpx.NotFound("role")
	}
	return result.Error
}

func makePage[T any](items []T, page, pageSize int, total int64) *domain.Page[T] {
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &domain.Page[T]{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
}
