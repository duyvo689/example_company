package service

import (
	"context"
	"strings"
	"time"

	"example-company/internal/domain"
	"example-company/internal/httpx"
	platformjwt "example-company/internal/platform/jwt"
	"example-company/internal/platform/password"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) Register(ctx context.Context, req domain.RegisterRequest) (*domain.AuthResponse, error) {
	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, err
	}
	var result *domain.AuthResponse
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		company := domain.Company{Name: strings.TrimSpace(req.CompanyName), Status: domain.StatusActive}
		user := domain.User{Email: strings.ToLower(strings.TrimSpace(req.Email)), Name: strings.TrimSpace(req.Name), PasswordHash: hash, Status: domain.StatusActive}
		if err := tx.Create(&company).Error; err != nil {
			return translate(err, "company")
		}
		if err := tx.Create(&user).Error; err != nil {
			return translate(err, "user")
		}
		role, err := ensureRole(tx, "ADMIN", domain.ScopeCompany, "Company administrator")
		if err != nil {
			return err
		}
		membership := domain.Membership{CompanyID: company.ID, UserID: user.ID, RoleID: role.ID, Status: domain.StatusActive, Company: &company, User: &user, Role: role}
		if err := tx.Create(&membership).Error; err != nil {
			return translate(err, "membership")
		}
		result, err = s.issuePair(tx, user, membership)
		return err
	})
	return result, err
}

func (s *Service) Login(ctx context.Context, req domain.LoginRequest) (*domain.AuthResponse, error) {
	var user domain.User
	if err := s.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil {
		return nil, httpx.Unauthorized("invalid email or password")
	}
	if user.Status != domain.StatusActive || !password.Verify(user.PasswordHash, req.Password) {
		return nil, httpx.Unauthorized("invalid email or password")
	}
	query := s.db.WithContext(ctx).Preload("Company").Preload("User").Preload("Role").
		Where("user_id = ? AND status = ?", user.ID, domain.StatusActive)
	if req.CompanyID != nil {
		query = query.Where("company_id = ?", *req.CompanyID)
	}
	var membership domain.Membership
	if err := query.Order("created_at ASC").First(&membership).Error; err != nil {
		return nil, httpx.Unauthorized("no active membership found")
	}
	if membership.Company == nil || membership.Company.Status != domain.StatusActive {
		return nil, httpx.Forbidden("company is inactive")
	}
	return s.issuePair(s.db.WithContext(ctx), user, membership)
}

func (s *Service) Refresh(ctx context.Context, raw string) (*domain.AuthResponse, error) {
	claims, err := s.tokens.Parse(raw, platformjwt.RefreshToken)
	if err != nil {
		return nil, httpx.Unauthorized("invalid or expired refresh token")
	}
	var result *domain.AuthResponse
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored domain.RefreshToken
		if err := tx.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", platformjwt.Hash(raw), time.Now()).First(&stored).Error; err != nil {
			return httpx.Unauthorized("refresh token has been revoked")
		}
		now := time.Now()
		if err := tx.Model(&stored).Update("revoked_at", now).Error; err != nil {
			return err
		}
		var membership domain.Membership
		if err := tx.Preload("Company").Preload("User").Preload("Role").Where("id = ? AND user_id = ? AND status = ?", claims.MembershipID, claims.UserID, domain.StatusActive).First(&membership).Error; err != nil {
			return httpx.Unauthorized("membership is no longer active")
		}
		if membership.User == nil || membership.User.Status != domain.StatusActive || membership.Company == nil || membership.Company.Status != domain.StatusActive {
			return httpx.Unauthorized("account is inactive")
		}
		result, err = s.issuePair(tx, *membership.User, membership)
		return err
	})
	return result, err
}

func (s *Service) Logout(ctx context.Context, raw string) error {
	return s.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("token_hash = ? AND revoked_at IS NULL", platformjwt.Hash(raw)).Update("revoked_at", time.Now()).Error
}

func (s *Service) Me(ctx context.Context, claims *platformjwt.Claims) (*domain.Membership, error) {
	var membership domain.Membership
	err := s.db.WithContext(ctx).Preload("Company").Preload("User").Preload("Role").First(&membership, "id = ? AND company_id = ?", claims.MembershipID, claims.CompanyID).Error
	return &membership, translate(err, "membership")
}

func (s *Service) UpdateMe(ctx context.Context, claims *platformjwt.Claims, name string) (*domain.User, error) {
	var user domain.User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&user, "id = ?", claims.UserID).Error; err != nil {
			return translate(err, "user")
		}
		user.Name = strings.TrimSpace(name)
		return tx.Save(&user).Error
	})
	return &user, err
}

func (s *Service) ChangePassword(ctx context.Context, claims *platformjwt.Claims, currentPassword, newPassword string) error {
	var user domain.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", claims.UserID).Error; err != nil {
		return translate(err, "user")
	}
	if !password.Verify(user.PasswordHash, currentPassword) {
		return httpx.Unauthorized("current password is incorrect")
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Update("password_hash", hash).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&domain.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error
	})
}

func (s *Service) issuePair(tx *gorm.DB, user domain.User, membership domain.Membership) (*domain.AuthResponse, error) {
	if membership.Role == nil {
		var role domain.Role
		if err := tx.First(&role, "id = ?", membership.RoleID).Error; err != nil {
			return nil, err
		}
		membership.Role = &role
	}
	access, refresh, refreshClaims, err := s.tokens.Pair(user.ID, membership.ID, membership.CompanyID, membership.Role.Code, membership.Role.Scope)
	if err != nil {
		return nil, err
	}
	expiresAt := refreshClaims.ExpiresAt.Time
	stored := domain.RefreshToken{ID: uuid.MustParse(refreshClaims.ID), UserID: user.ID, TokenHash: platformjwt.Hash(refresh), ExpiresAt: expiresAt}
	if err := tx.Create(&stored).Error; err != nil {
		return nil, err
	}
	membership.User = &user
	return &domain.AuthResponse{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(s.tokens.AccessTTL().Seconds()), User: user, Membership: membership}, nil
}

func ensureRole(tx *gorm.DB, code, scope, description string) (*domain.Role, error) {
	var role domain.Role
	err := tx.Where("code = ?", code).First(&role).Error
	if err == nil {
		return &role, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	role = domain.Role{Code: code, Scope: scope, Description: description}
	if err := tx.Create(&role).Error; err != nil {
		// Handles a concurrent bootstrap.
		if findErr := tx.Where("code = ?", code).First(&role).Error; findErr == nil {
			return &role, nil
		}
		return nil, err
	}
	return &role, nil
}
