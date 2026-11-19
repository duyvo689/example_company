package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusPending  = "pending"
	ScopeCompany   = "company"
	ScopeLocation  = "location"
)

type BaseModel struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (m *BaseModel) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

type Company struct {
	BaseModel
	Name   string `json:"name" gorm:"size:180;not null"`
	Status string `json:"status" gorm:"size:30;not null;default:active;index"`
}

type Location struct {
	BaseModel
	CompanyID uuid.UUID `json:"company_id" gorm:"type:uuid;not null;uniqueIndex:idx_location_company_code"`
	Code      string    `json:"code" gorm:"size:50;not null;uniqueIndex:idx_location_company_code"`
	Name      string    `json:"name" gorm:"size:180;not null"`
	Status    string    `json:"status" gorm:"size:30;not null;default:active;index"`
	Company   *Company  `json:"company,omitempty" gorm:"foreignKey:CompanyID"`
}

type User struct {
	BaseModel
	Email        string `json:"email" gorm:"size:254;not null;uniqueIndex"`
	Name         string `json:"name" gorm:"size:180;not null"`
	PasswordHash string `json:"-" gorm:"size:100;not null"`
	Status       string `json:"status" gorm:"size:30;not null;default:active;index"`
}

type Role struct {
	BaseModel
	Code        string `json:"code" gorm:"size:50;not null;uniqueIndex"`
	Scope       string `json:"scope" gorm:"size:30;not null"`
	Description string `json:"description" gorm:"size:255"`
}

type Membership struct {
	BaseModel
	CompanyID uuid.UUID `json:"company_id" gorm:"type:uuid;not null;uniqueIndex:idx_membership_company_user"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_membership_company_user"`
	RoleID    uuid.UUID `json:"role_id" gorm:"type:uuid;not null;index"`
	Status    string    `json:"status" gorm:"size:30;not null;default:active;index"`
	Company   *Company  `json:"company,omitempty" gorm:"foreignKey:CompanyID"`
	User      *User     `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Role      *Role     `json:"role,omitempty" gorm:"foreignKey:RoleID"`
}

type Patient struct {
	BaseModel
	CompanyID         uuid.UUID  `json:"company_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_patient_company_ref"`
	PrimaryLocationID *uuid.UUID `json:"primary_location_id,omitempty" gorm:"type:uuid;index"`
	PublicRef         string     `json:"public_ref" gorm:"size:100;not null;uniqueIndex:idx_patient_company_ref"`
	ExternalChartID   string     `json:"external_chart_id" gorm:"size:100;index"`
	Status            string     `json:"status" gorm:"size:30;not null;default:active;index"`
	Company           *Company   `json:"company,omitempty" gorm:"foreignKey:CompanyID"`
	PrimaryLocation   *Location  `json:"primary_location,omitempty" gorm:"foreignKey:PrimaryLocationID"`
}

type PatientLocation struct {
	PatientID    uuid.UUID `json:"patient_id" gorm:"type:uuid;primaryKey"`
	LocationID   uuid.UUID `json:"location_id" gorm:"type:uuid;primaryKey"`
	Relationship string    `json:"relationship" gorm:"size:50;not null"`
	CreatedAt    time.Time `json:"created_at"`
	Patient      *Patient  `json:"patient,omitempty" gorm:"foreignKey:PatientID"`
	Location     *Location `json:"location,omitempty" gorm:"foreignKey:LocationID"`
}

type LocationAssignment struct {
	MembershipID uuid.UUID   `json:"membership_id" gorm:"type:uuid;primaryKey"`
	LocationID   uuid.UUID   `json:"location_id" gorm:"type:uuid;primaryKey"`
	CreatedAt    time.Time   `json:"created_at"`
	Membership   *Membership `json:"membership,omitempty" gorm:"foreignKey:MembershipID"`
	Location     *Location   `json:"location,omitempty" gorm:"foreignKey:LocationID"`
}

type Order struct {
	BaseModel
	CompanyID             uuid.UUID    `json:"company_id" gorm:"type:uuid;not null;index"`
	LocationID            uuid.UUID    `json:"location_id" gorm:"type:uuid;not null;index"`
	PatientID             uuid.UUID    `json:"patient_id" gorm:"type:uuid;not null;index"`
	CreatedByMembershipID uuid.UUID    `json:"created_by_membership_id" gorm:"type:uuid;not null;index"`
	JourneyStepID         *uuid.UUID   `json:"journey_step_id,omitempty" gorm:"type:uuid;index"`
	ExternalOrderID       string       `json:"external_order_id" gorm:"size:100;uniqueIndex:idx_order_company_external"`
	Status                string       `json:"status" gorm:"size:30;not null;default:pending;index"`
	Company               *Company     `json:"company,omitempty" gorm:"foreignKey:CompanyID"`
	Location              *Location    `json:"location,omitempty" gorm:"foreignKey:LocationID"`
	Patient               *Patient     `json:"patient,omitempty" gorm:"foreignKey:PatientID"`
	CreatedBy             *Membership  `json:"created_by,omitempty" gorm:"foreignKey:CreatedByMembershipID"`
	JourneyStep           *JourneyStep `json:"journey_step,omitempty" gorm:"foreignKey:JourneyStepID"`
}

type AuditLog struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	CompanyID    *uuid.UUID     `json:"company_id,omitempty" gorm:"type:uuid;index"`
	MembershipID *uuid.UUID     `json:"membership_id,omitempty" gorm:"type:uuid;index"`
	RequestID    string         `json:"request_id" gorm:"size:64;index"`
	Method       string         `json:"method" gorm:"size:10;not null"`
	Path         string         `json:"path" gorm:"size:500;not null"`
	Action       string         `json:"action" gorm:"size:50;not null;index"`
	StatusCode   int            `json:"status_code"`
	IPAddress    string         `json:"ip_address" gorm:"size:64"`
	UserAgent    string         `json:"user_agent" gorm:"size:500"`
	Metadata     datatypes.JSON `json:"metadata" gorm:"type:jsonb"`
	CreatedAt    time.Time      `json:"created_at" gorm:"index"`
}

func (m *AuditLog) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

type IntegrationEvent struct {
	ID            uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	CompanyID     uuid.UUID      `json:"company_id" gorm:"type:uuid;not null;index"`
	AggregateType string         `json:"aggregate_type" gorm:"size:50;not null"`
	AggregateID   uuid.UUID      `json:"aggregate_id" gorm:"type:uuid;not null;index"`
	EventType     string         `json:"event_type" gorm:"size:100;not null;index"`
	Payload       datatypes.JSON `json:"payload" gorm:"type:jsonb;not null"`
	Status        string         `json:"status" gorm:"size:30;not null;default:pending;index"`
	Attempts      int            `json:"attempts" gorm:"not null;default:0"`
	LastError     string         `json:"last_error,omitempty" gorm:"type:text"`
	NextAttemptAt time.Time      `json:"next_attempt_at" gorm:"index"`
	PublishedAt   *time.Time     `json:"published_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func (m *IntegrationEvent) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.NextAttemptAt.IsZero() {
		m.NextAttemptAt = time.Now()
	}
	return nil
}

type RefreshToken struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	TokenHash string     `json:"-" gorm:"size:64;not null;uniqueIndex"`
	ExpiresAt time.Time  `json:"expires_at" gorm:"index"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (m *RefreshToken) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
