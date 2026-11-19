package domain

import (
	"time"

	"github.com/google/uuid"
)

type JourneyStepRequest struct {
	AssignedMembershipID *uuid.UUID     `json:"assigned_membership_id"`
	StepKey              string         `json:"step_key" binding:"required,min=1,max=100"`
	StepType             string         `json:"step_type" binding:"required,min=1,max=50"`
	CycleNumber          int            `json:"cycle_number" binding:"omitempty,min=1"`
	Status               string         `json:"status" binding:"omitempty,oneof=PENDING NOT_STARTED READY WAITING_PATIENT IN_PROGRESS IN_REVIEW WAITING_DOCTOR BLOCKED SUBMITTED COMPLETED SKIPPED EXPIRED REJECTED CANCELLED"`
	DueAt                *time.Time     `json:"due_at"`
	Metadata             map[string]any `json:"metadata"`
}

type CreateCareJourneyRequest struct {
	PatientID     uuid.UUID            `json:"patient_id" binding:"required"`
	LocationID    uuid.UUID            `json:"location_id" binding:"required"`
	ProgramCode   string               `json:"program_code" binding:"required,min=1,max=50"`
	EpisodeNumber int                  `json:"episode_number" binding:"omitempty,min=1"`
	Status        string               `json:"status" binding:"omitempty,oneof=DRAFT ONBOARDING ACTIVE PAUSED COMPLETED CHURNED CANCELLED"`
	CurrentStage  string               `json:"current_stage" binding:"omitempty,max=50"`
	CurrentCycle  int                  `json:"current_cycle" binding:"omitempty,min=1"`
	StartedAt     *time.Time           `json:"started_at"`
	InitialSteps  []JourneyStepRequest `json:"initial_steps" binding:"omitempty,dive"`
}

type UpdateCareJourneyRequest struct {
	LocationID   *uuid.UUID `json:"location_id"`
	Status       *string    `json:"status" binding:"omitempty,oneof=DRAFT ONBOARDING ACTIVE PAUSED COMPLETED CHURNED CANCELLED"`
	CurrentStage *string    `json:"current_stage" binding:"omitempty,max=50"`
	CurrentCycle *int       `json:"current_cycle" binding:"omitempty,min=1"`
}

type UpdateJourneyStepRequest struct {
	AssignedMembershipID *uuid.UUID     `json:"assigned_membership_id"`
	Status               *string        `json:"status" binding:"omitempty,oneof=PENDING NOT_STARTED READY WAITING_PATIENT IN_PROGRESS IN_REVIEW WAITING_DOCTOR BLOCKED SUBMITTED COMPLETED SKIPPED EXPIRED REJECTED CANCELLED"`
	DueAt                *time.Time     `json:"due_at"`
	Metadata             map[string]any `json:"metadata"`
	JourneyStage         *string        `json:"journey_stage" binding:"omitempty,max=50"`
	Note                 string         `json:"note" binding:"max=2000"`
}

type CreateIntakeSubmissionRequest struct {
	FormVersionID *uuid.UUID     `json:"form_version_id"`
	PatientID     *uuid.UUID     `json:"patient_id"`
	LocationID    uuid.UUID      `json:"location_id" binding:"required"`
	JourneyID     *uuid.UUID     `json:"journey_id"`
	JourneyStepID *uuid.UUID     `json:"journey_step_id"`
	IntakeType    string         `json:"intake_type" binding:"required,oneof=NEW_PATIENT_INTAKE TRT_INTAKE GLP1_INTAKE TRT_RENEWAL GLP1_RENEWAL FOLLOW_UP ADDRESS_UPDATE"`
	SchemaVersion int            `json:"schema_version" binding:"required,min=1"`
	Status        string         `json:"status" binding:"omitempty,oneof=SUBMITTED VALIDATED REVIEWED REJECTED"`
	Answers       map[string]any `json:"answers" binding:"required"`
	ClinicalFlags map[string]any `json:"clinical_flags"`
}

type CreateIntakeFileRequest struct {
	FileType   string `json:"file_type" binding:"required,min=1,max=50"`
	StorageKey string `json:"storage_key" binding:"required,min=1,max=500"`
	MimeType   string `json:"mime_type" binding:"required,min=1,max=100"`
}

type IssueRenewalLinkRequest struct {
	VerificationValue string     `json:"verification_value" binding:"required,min=4,max=200"`
	ExpiresAt         *time.Time `json:"expires_at"`
	ExpiresInDays     int        `json:"expires_in_days" binding:"omitempty,min=1,max=90"`
	MaxAttempts       int        `json:"max_attempts" binding:"omitempty,min=1,max=10"`
}

type IssueRenewalLinkResponse struct {
	RenewalLink RenewalLink `json:"renewal_link"`
	Token       string      `json:"token"`
}

type OpenRenewalLinkRequest struct {
	VerificationValue string `json:"verification_value" binding:"required"`
}

type RenewalLinkPublicResponse struct {
	Status        string         `json:"status"`
	ExpiresAt     time.Time      `json:"expires_at"`
	JourneyStepID uuid.UUID      `json:"journey_step_id"`
	StepType      string         `json:"step_type"`
	IntakeType    string         `json:"intake_type"`
	FormVersionID *uuid.UUID     `json:"form_version_id,omitempty"`
	FormSchema    map[string]any `json:"form_schema,omitempty"`
}

type SubmitRenewalRequest struct {
	VerificationValue string         `json:"verification_value" binding:"required"`
	IntakeType        string         `json:"intake_type" binding:"required,oneof=TRT_RENEWAL GLP1_RENEWAL FOLLOW_UP"`
	SchemaVersion     int            `json:"schema_version" binding:"required,min=1"`
	Answers           map[string]any `json:"answers" binding:"required"`
	ClinicalFlags     map[string]any `json:"clinical_flags"`
}

type ChurnEventRequest struct {
	EventType  string `json:"event_type" binding:"required,oneof=CHURNED REACTIVATED TRANSFERRED COMPLETED CLOSED"`
	ReasonCode string `json:"reason_code" binding:"required,min=1,max=50"`
	Note       string `json:"note" binding:"max=2000"`
	NextStage  string `json:"next_stage" binding:"omitempty,max=50"`
}

type CareJourneyDetail struct {
	Journey       CareJourney         `json:"journey"`
	Steps         []JourneyStep       `json:"steps"`
	ChurnEvents   []PatientChurnEvent `json:"churn_events"`
	StatusHistory []StatusHistory     `json:"status_history"`
}
