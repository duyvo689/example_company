package domain

import (
	"time"

	"github.com/google/uuid"
)

type TreatmentProgramRequest struct {
	Code        string `json:"code" binding:"required,min=1,max=50"`
	Name        string `json:"name" binding:"required,min=2,max=180"`
	Category    string `json:"category" binding:"required,oneof=DENTAL AESTHETIC GENERAL"`
	Description string `json:"description" binding:"max=5000"`
	Status      string `json:"status" binding:"omitempty,oneof=active inactive"`
}

type ProgramVersionRequest struct {
	JourneyWorkflowID *uuid.UUID `json:"journey_workflow_id"`
	ChangeNote        string     `json:"change_note" binding:"max=2000"`
}

type StepTemplateRequest struct {
	Code               string         `json:"code" binding:"required,min=1,max=100"`
	Name               string         `json:"name" binding:"required,min=2,max=180"`
	StepType           string         `json:"step_type" binding:"required,min=1,max=50"`
	DisplayOrder       int            `json:"display_order" binding:"required,min=1"`
	IsRequired         *bool          `json:"is_required"`
	AssignedRoleID     *uuid.UUID     `json:"assigned_role_id"`
	DueDays            *int           `json:"due_days" binding:"omitempty,min=0,max=3650"`
	CreatesOrder       bool           `json:"creates_order"`
	RepeatType         string         `json:"repeat_type" binding:"omitempty,oneof=NONE INTERVAL"`
	RepeatIntervalDays *int           `json:"repeat_interval_days" binding:"omitempty,min=1,max=3650"`
	MaxOccurrences     *int           `json:"max_occurrences" binding:"omitempty,min=1,max=1000"`
	Condition          map[string]any `json:"condition"`
	WorkflowTemplateID *uuid.UUID     `json:"workflow_template_id"`
}

type StepDependencyRequest struct {
	DependsOnStepID uuid.UUID      `json:"depends_on_step_id" binding:"required"`
	Condition       map[string]any `json:"condition"`
}

type FormTemplateRequest struct {
	Code        string `json:"code" binding:"required,min=1,max=100"`
	Name        string `json:"name" binding:"required,min=2,max=180"`
	Description string `json:"description" binding:"max=5000"`
	Status      string `json:"status" binding:"omitempty,oneof=active inactive"`
}

type FormVersionRequest struct {
	Schema map[string]any `json:"schema" binding:"required"`
}

type StepFormAssignmentRequest struct {
	FormVersionID uuid.UUID `json:"form_version_id" binding:"required"`
	Phase         string    `json:"phase" binding:"required,oneof=BEFORE DURING AFTER"`
	ActorType     string    `json:"actor_type" binding:"required,oneof=PATIENT STAFF DOCTOR"`
	IsRequired    *bool     `json:"is_required"`
}

type WorkflowStateRequest struct {
	Code         string `json:"code" binding:"required,min=1,max=50"`
	StandardCode string `json:"standard_code" binding:"required,min=1,max=50"`
	DisplayName  string `json:"display_name" binding:"required,min=1,max=100"`
	DisplayOrder int    `json:"display_order" binding:"required,min=1"`
	IsInitial    bool   `json:"is_initial"`
	IsTerminal   bool   `json:"is_terminal"`
}

type WorkflowTransitionRequest struct {
	FromStateCode  string     `json:"from_state_code" binding:"required"`
	ToStateCode    string     `json:"to_state_code" binding:"required"`
	RequiredRoleID *uuid.UUID `json:"required_role_id"`
}

type WorkflowTemplateRequest struct {
	Code        string                      `json:"code" binding:"required,min=1,max=100"`
	Name        string                      `json:"name" binding:"required,min=2,max=180"`
	EntityType  string                      `json:"entity_type" binding:"required,oneof=JOURNEY STEP"`
	States      []WorkflowStateRequest      `json:"states" binding:"required,min=2,dive"`
	Transitions []WorkflowTransitionRequest `json:"transitions" binding:"required,min=1,dive"`
}

type ProgramLocationRequest struct {
	LocationID uuid.UUID      `json:"location_id" binding:"required"`
	Status     string         `json:"status" binding:"omitempty,oneof=active inactive"`
	Config     map[string]any `json:"config"`
}

type InstantiateProgramRequest struct {
	PatientID  uuid.UUID      `json:"patient_id" binding:"required"`
	LocationID uuid.UUID      `json:"location_id" binding:"required"`
	Context    map[string]any `json:"context"`
	StartedAt  *time.Time     `json:"started_at"`
}

type TreatmentProgramDetail struct {
	Program   TreatmentProgram  `json:"program"`
	Versions  []ProgramVersion  `json:"versions"`
	Locations []ProgramLocation `json:"locations"`
}
