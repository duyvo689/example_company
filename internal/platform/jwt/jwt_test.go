package jwt

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPairAndParse(t *testing.T) {
	manager := New("this-is-a-test-secret-with-at-least-32-characters", time.Minute, time.Hour)
	userID, membershipID, companyID := uuid.New(), uuid.New(), uuid.New()
	access, refresh, _, err := manager.Pair(userID, membershipID, companyID, "ADMIN", "company")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Parse(access, AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != userID || claims.MembershipID != membershipID || claims.CompanyID != companyID {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := manager.Parse(refresh, AccessToken); err == nil {
		t.Fatal("refresh token must not be accepted as access token")
	}
}

func TestHashIsStable(t *testing.T) {
	if Hash("token") != Hash("token") || Hash("token") == Hash("other") {
		t.Fatal("unexpected token hash")
	}
}
