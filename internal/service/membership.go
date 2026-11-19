package service

import (
	"context"
	"strings"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	"example-company/internal/platform/password"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) ListMemberships(ctx context.Context, companyID uuid.UUID, page, pageSize int, search, status string) (*domain.Page[domain.Membership], error) {
	base := s.db.WithContext(ctx).Model(&domain.Membership{}).Where("memberships.company_id = ?", companyID)
	if search != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		base = base.Joins("JOIN users u ON u.id = memberships.user_id").Where("LOWER(u.email) LIKE ? OR LOWER(u.name) LIKE ?", like, like)
	}
	if status != "" {
		base = base.Where("memberships.status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []domain.Membership
	if err := base.Preload("User").Preload("Role").Order("memberships.created_at DESC").Offset(pageOffset(page, pageSize)).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return makePage(items, page, pageSize, total), nil
}

func (s *Service) GetMembership(ctx context.Context, companyID, id uuid.UUID) (*domain.Membership, error) {
	var membership domain.Membership
	err := s.db.WithContext(ctx).Preload("User").Preload("Role").Preload("Company").First(&membership, "id = ? AND company_id = ?", id, companyID).Error
	return &membership, translate(err, "membership")
}

func (s *Service) CreateMembership(ctx context.Context, companyID uuid.UUID, req domain.MembershipRequest) (*domain.Membership, error) {
	var membership domain.Membership
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user domain.User
		if err := tx.Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return err
			}
			if strings.TrimSpace(req.Name) == "" || req.Password == "" {
				return httpx.BadRequest("name and password are required when provisioning a new user")
			}
			hash, hashErr := password.Hash(req.Password)
			if hashErr != nil {
				return hashErr
			}
			user = domain.User{Email: strings.ToLower(strings.TrimSpace(req.Email)), Name: strings.TrimSpace(req.Name), PasswordHash: hash, Status: domain.StatusActive}
			if createErr := tx.Create(&user).Error; createErr != nil {
				return translate(createErr, "user")
			}
		}
		var role domain.Role
		if err := tx.First(&role, "id = ?", req.RoleID).Error; err != nil {
			return httpx.NotFound("role")
		}
		status := req.Status
		if status == "" {
			status = domain.StatusActive
		}
		membership = domain.Membership{CompanyID: companyID, UserID: user.ID, RoleID: role.ID, Status: status, User: &user, Role: &role}
		return translate(tx.Create(&membership).Error, "membership")
	})
	return &membership, err
}

func (s *Service) UpdateMembership(ctx context.Context, companyID, id uuid.UUID, req domain.MembershipUpdateRequest) (*domain.Membership, error) {
	existing, err := s.GetMembership(ctx, companyID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	newRoleCode := existing.Role.Code
	newStatus := existing.Status
	if req.RoleID != nil {
		var role domain.Role
		if err := s.db.WithContext(ctx).First(&role, "id = ?", *req.RoleID).Error; err != nil {
			return nil, translate(err, "role")
		}
		newRoleCode = role.Code
		updates["role_id"] = *req.RoleID
	}
	if req.Status != nil {
		newStatus = *req.Status
		updates["status"] = *req.Status
	}
	if existing.Role.Code == "ADMIN" && (newRoleCode != "ADMIN" || newStatus != domain.StatusActive) {
		if err := ensureAnotherAdmin(s.db.WithContext(ctx), companyID, id); err != nil {
			return nil, err
		}
	}
	if len(updates) == 0 {
		return existing, nil
	}
	result := s.db.WithContext(ctx).Model(&domain.Membership{}).Where("id = ? AND company_id = ?", id, companyID).Updates(updates)
	if result.Error != nil {
		return nil, translate(result.Error, "membership")
	}
	if result.RowsAffected == 0 {
		return nil, httpx.NotFound("membership")
	}
	return s.GetMembership(ctx, companyID, id)
}

func (s *Service) DeleteMembership(ctx context.Context, companyID, id, actorMembershipID uuid.UUID) error {
	if id == actorMembershipID {
		return httpx.BadRequest("you cannot delete your own active membership")
	}
	var membership domain.Membership
	if err := s.db.WithContext(ctx).Preload("Role").First(&membership, "id = ? AND company_id = ?", id, companyID).Error; err != nil {
		return translate(err, "membership")
	}
	if membership.Role != nil && membership.Role.Code == "ADMIN" && membership.Status == domain.StatusActive {
		if err := ensureAnotherAdmin(s.db.WithContext(ctx), companyID, id); err != nil {
			return err
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var orders int64
		if err := tx.Model(&domain.Order{}).Where("created_by_membership_id = ?", id).Count(&orders).Error; err != nil {
			return err
		}
		if orders > 0 {
			return httpx.Conflict("membership is referenced by orders; set it inactive instead")
		}
		if err := tx.Where("membership_id = ?", id).Delete(&domain.LocationAssignment{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND company_id = ?", id, companyID).Delete(&domain.Membership{})
		if result.RowsAffected == 0 {
			return httpx.NotFound("membership")
		}
		return result.Error
	})
}

func ensureAnotherAdmin(db *gorm.DB, companyID, excludedMembershipID uuid.UUID) error {
	var count int64
	err := db.Table("memberships m").Joins("JOIN roles r ON r.id = m.role_id").
		Where("m.company_id = ? AND m.id <> ? AND m.status = ? AND m.deleted_at IS NULL AND r.code = ? AND r.deleted_at IS NULL", companyID, excludedMembershipID, domain.StatusActive, "ADMIN").
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return httpx.Conflict("company must keep at least one active administrator")
	}
	return nil
}

func (s *Service) ListAssignments(ctx context.Context, companyID, membershipID uuid.UUID) ([]domain.LocationAssignment, error) {
	if _, err := s.GetMembership(ctx, companyID, membershipID); err != nil {
		return nil, err
	}
	var items []domain.LocationAssignment
	err := s.db.WithContext(ctx).Preload("Location").Where("membership_id = ?", membershipID).Find(&items).Error
	return items, err
}

func (s *Service) AddAssignment(ctx context.Context, companyID, membershipID, locationID uuid.UUID) (*domain.LocationAssignment, error) {
	if _, err := s.GetMembership(ctx, companyID, membershipID); err != nil {
		return nil, err
	}
	if err := validLocation(s.db.WithContext(ctx), companyID, locationID); err != nil {
		return nil, err
	}
	assignment := domain.LocationAssignment{MembershipID: membershipID, LocationID: locationID}
	err := s.db.WithContext(ctx).Create(&assignment).Error
	return &assignment, translate(err, "location assignment")
}

func (s *Service) DeleteAssignment(ctx context.Context, companyID, membershipID, locationID uuid.UUID) error {
	if _, err := s.GetMembership(ctx, companyID, membershipID); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Where("membership_id = ? AND location_id = ?", membershipID, locationID).Delete(&domain.LocationAssignment{})
	if result.RowsAffected == 0 {
		return httpx.NotFound("location assignment")
	}
	return result.Error
}
