package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	JourneyStatusDraft      = "DRAFT"
	JourneyStatusOnboarding = "ONBOARDING"
	JourneyStatusActive     = "ACTIVE"
	JourneyStatusPaused     = "PAUSED"
	JourneyStatusCompleted  = "COMPLETED"
	JourneyStatusChurned    = "CHURNED"
	JourneyStatusCancelled  = "CANCELLED"

	StepStatusPending        = "PENDING"
	StepStatusWaitingPatient = "WAITING_PATIENT"
	StepStatusInReview       = "IN_REVIEW"
	StepStatusReady          = "READY"
	StepStatusSubmitted      = "SUBMITTED"
	StepStatusCompleted      = "COMPLETED"
	StepStatusExpired        = "EXPIRED"
	StepStatusRejected       = "REJECTED"
	StepStatusCancelled      = "CANCELLED"

	RenewalStatusIssued    = "ISSUED"
	RenewalStatusOpened    = "OPENED"
	RenewalStatusSubmitted = "SUBMITTED"
	RenewalStatusCompleted = "COMPLETED"
	RenewalStatusExpired   = "EXPIRED"
	RenewalStatusRevoked   = "REVOKED"
	RenewalStatusLocked    = "LOCKED"
)

type CareJourney struct {
	BaseModel
	CompanyID             uuid.UUID       `json:"company_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_journey_episode"`
	ProgramVersionID      *uuid.UUID      `json:"program_version_id,omitempty" gorm:"type:uuid;index"`
	PatientID             uuid.UUID       `json:"patient_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_journey_episode"`
	LocationID            uuid.UUID       `json:"location_id" gorm:"type:uuid;not null;index"`
	CreatedByMembershipID uuid.UUID       `json:"created_by_membership_id" gorm:"type:uuid;not null;index"`
	ProgramCode           string          `json:"program_code" gorm:"size:50;not null;uniqueIndex:idx_journey_episode"`
	EpisodeNumber         int             `json:"episode_number" gorm:"not null;uniqueIndex:idx_journey_episode"`
	Status                string          `json:"status" gorm:"size:30;not null;index"`
	CurrentStage          string          `json:"current_stage" gorm:"size:50;not null;index"`
	CurrentCycle          int             `json:"current_cycle" gorm:"not null;default:1"`
	ContextData           datatypes.JSON  `json:"context_data" gorm:"type:jsonb;not null;default:'{}'"`
	StartedAt             time.Time       `json:"started_at"`
	EndedAt               *time.Time      `json:"ended_at,omitempty"`
	Company               *Company        `json:"company,omitempty" gorm:"foreignKey:CompanyID"`
	Patient               *Patient        `json:"patient,omitempty" gorm:"foreignKey:PatientID"`
	Location              *Location       `json:"location,omitempty" gorm:"foreignKey:LocationID"`
	CreatedBy             *Membership     `json:"created_by,omitempty" gorm:"foreignKey:CreatedByMembershipID"`
	ProgramVersion        *ProgramVersion `json:"program_version,omitempty" gorm:"foreignKey:ProgramVersionID"`
	Steps                 []JourneyStep   `json:"steps,omitempty" gorm:"foreignKey:JourneyID"`
}

type JourneyStep struct {
	BaseModel
	CompanyID            uuid.UUID      `json:"company_id" gorm:"type:uuid;not null;index"`
	StepTemplateID       *uuid.UUID     `json:"step_template_id,omitempty" gorm:"type:uuid;index"`
	JourneyID            uuid.UUID      `json:"journey_id" gorm:"type:uuid;not null;uniqueIndex:idx_journey_step_key"`
	AssignedMembershipID *uuid.UUID     `json:"assigned_membership_id,omitempty" gorm:"type:uuid;index"`
	StepKey              string         `json:"step_key" gorm:"size:100;not null;uniqueIndex:idx_journey_step_key"`
	StepType             string         `json:"step_type" gorm:"size:50;not null;index"`
	CycleNumber          int            `json:"cycle_number" gorm:"not null;default:1"`
	TemplateIteration    int            `json:"template_iteration" gorm:"not null;default:1"`
	Status               string         `json:"status" gorm:"size:30;not null;index"`
	DueAt                *time.Time     `json:"due_at,omitempty"`
	CompletedAt          *time.Time     `json:"completed_at,omitempty"`
	Metadata             datatypes.JSON `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	Journey              *CareJourney   `json:"journey,omitempty" gorm:"foreignKey:JourneyID"`
	StepTemplate         *StepTemplate  `json:"step_template,omitempty" gorm:"foreignKey:StepTemplateID"`
	AssignedMembership   *Membership    `json:"assigned_membership,omitempty" gorm:"foreignKey:AssignedMembershipID"`
}

type RenewalLink struct {
	BaseModel
	CompanyID        uuid.UUID    `json:"company_id" gorm:"type:uuid;not null;index"`
	JourneyStepID    uuid.UUID    `json:"journey_step_id" gorm:"type:uuid;not null;index"`
	TokenHash        string       `json:"-" gorm:"size:64;not null;uniqueIndex"`
	VerificationHash string       `json:"-" gorm:"size:100;not null"`
	Status           string       `json:"status" gorm:"size:30;not null;index"`
	AttemptCount     int          `json:"attempt_count" gorm:"not null;default:0"`
	MaxAttempts      int          `json:"max_attempts" gorm:"not null;default:5"`
	ExpiresAt        time.Time    `json:"expires_at" gorm:"index"`
	OpenedAt         *time.Time   `json:"opened_at,omitempty"`
	SubmittedAt      *time.Time   `json:"submitted_at,omitempty"`
	RevokedAt        *time.Time   `json:"revoked_at,omitempty"`
	JourneyStep      *JourneyStep `json:"journey_step,omitempty" gorm:"foreignKey:JourneyStepID"`
}

type IntakeSubmission struct {
	BaseModel
	CompanyID     uuid.UUID      `json:"company_id" gorm:"type:uuid;not null;index"`
	FormVersionID *uuid.UUID     `json:"form_version_id,omitempty" gorm:"type:uuid;index"`
	PatientID     *uuid.UUID     `json:"patient_id,omitempty" gorm:"type:uuid;index"`
	LocationID    uuid.UUID      `json:"location_id" gorm:"type:uuid;not null;index"`
	JourneyID     *uuid.UUID     `json:"journey_id,omitempty" gorm:"type:uuid;index"`
	JourneyStepID *uuid.UUID     `json:"journey_step_id,omitempty" gorm:"type:uuid;index"`
	RenewalLinkID *uuid.UUID     `json:"renewal_link_id,omitempty" gorm:"type:uuid;uniqueIndex"`
	IntakeType    string         `json:"intake_type" gorm:"size:50;not null;index"`
	SchemaVersion int            `json:"schema_version" gorm:"not null"`
	Status        string         `json:"status" gorm:"size:30;not null;index"`
	Answers       datatypes.JSON `json:"answers" gorm:"type:jsonb;not null"`
	ClinicalFlags datatypes.JSON `json:"clinical_flags" gorm:"type:jsonb;not null;default:'{}'"`
	SubmittedAt   time.Time      `json:"submitted_at" gorm:"index"`
	Patient       *Patient       `json:"patient,omitempty" gorm:"foreignKey:PatientID"`
	Location      *Location      `json:"location,omitempty" gorm:"foreignKey:LocationID"`
	Journey       *CareJourney   `json:"journey,omitempty" gorm:"foreignKey:JourneyID"`
	JourneyStep   *JourneyStep   `json:"journey_step,omitempty" gorm:"foreignKey:JourneyStepID"`
	RenewalLink   *RenewalLink   `json:"renewal_link,omitempty" gorm:"foreignKey:RenewalLinkID"`
	FormVersion   *FormVersion   `json:"form_version,omitempty" gorm:"foreignKey:FormVersionID"`
	Files         []IntakeFile   `json:"files,omitempty" gorm:"foreignKey:SubmissionID"`
}

type IntakeFile struct {
	BaseModel
	CompanyID    uuid.UUID         `json:"company_id" gorm:"type:uuid;not null;index"`
	SubmissionID uuid.UUID         `json:"submission_id" gorm:"type:uuid;not null;index"`
	FileType     string            `json:"file_type" gorm:"size:50;not null"`
	StorageKey   string            `json:"storage_key" gorm:"size:500;not null;uniqueIndex"`
	MimeType     string            `json:"mime_type" gorm:"size:100;not null"`
	Submission   *IntakeSubmission `json:"submission,omitempty" gorm:"foreignKey:SubmissionID"`
}

type PatientChurnEvent struct {
	ID                      uuid.UUID    `json:"id" gorm:"type:uuid;primaryKey"`
	CompanyID               uuid.UUID    `json:"company_id" gorm:"type:uuid;not null;index"`
	JourneyID               uuid.UUID    `json:"journey_id" gorm:"type:uuid;not null;index"`
	PatientID               uuid.UUID    `json:"patient_id" gorm:"type:uuid;not null;index"`
	LocationID              uuid.UUID    `json:"location_id" gorm:"type:uuid;not null;index"`
	PerformedByMembershipID uuid.UUID    `json:"performed_by_membership_id" gorm:"type:uuid;not null;index"`
	EventType               string       `json:"event_type" gorm:"size:30;not null;index"`
	StageAtEvent            string       `json:"stage_at_event" gorm:"size:50;not null"`
	ReasonCode              string       `json:"reason_code" gorm:"size:50;not null"`
	Note                    string       `json:"note" gorm:"type:text"`
	OccurredAt              time.Time    `json:"occurred_at" gorm:"index"`
	Journey                 *CareJourney `json:"journey,omitempty" gorm:"foreignKey:JourneyID"`
	PerformedBy             *Membership  `json:"performed_by,omitempty" gorm:"foreignKey:PerformedByMembershipID"`
}

func (m *PatientChurnEvent) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.OccurredAt.IsZero() {
		m.OccurredAt = time.Now()
	}
	return nil
}
