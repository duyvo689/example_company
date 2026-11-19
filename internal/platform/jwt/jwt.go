package jwt

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessToken  = "access"
	RefreshToken = "refresh"
)

type Claims struct {
	UserID       uuid.UUID `json:"user_id"`
	MembershipID uuid.UUID `json:"membership_id"`
	CompanyID    uuid.UUID `json:"company_id"`
	RoleCode     string    `json:"role_code"`
	Scope        string    `json:"scope"`
	TokenType    string    `json:"token_type"`
	jwtlib.RegisteredClaims
}

type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func New(secret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *Manager) Pair(userID, membershipID, companyID uuid.UUID, roleCode, scope string) (string, string, *Claims, error) {
	accessClaims := m.claims(userID, membershipID, companyID, roleCode, scope, AccessToken, m.accessTTL)
	refreshClaims := m.claims(userID, membershipID, companyID, roleCode, scope, RefreshToken, m.refreshTTL)
	access, err := m.sign(accessClaims)
	if err != nil {
		return "", "", nil, err
	}
	refresh, err := m.sign(refreshClaims)
	if err != nil {
		return "", "", nil, err
	}
	return access, refresh, refreshClaims, nil
}

func (m *Manager) Parse(raw, expectedType string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(raw, &Claims{}, func(token *jwtlib.Token) (any, error) {
		if token.Method != jwtlib.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	}, jwtlib.WithIssuer("example-company-api"), jwtlib.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.TokenType != expectedType {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) claims(userID, membershipID, companyID uuid.UUID, roleCode, scope, tokenType string, ttl time.Duration) *Claims {
	now := time.Now()
	return &Claims{
		UserID: userID, MembershipID: membershipID, CompanyID: companyID,
		RoleCode: roleCode, Scope: scope, TokenType: tokenType,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer: "example-company-api", Subject: userID.String(), ID: uuid.NewString(),
			IssuedAt: jwtlib.NewNumericDate(now), ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
		},
	}
}

func (m *Manager) sign(claims *Claims) (string, error) {
	return jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(m.secret)
}
