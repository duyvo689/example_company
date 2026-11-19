package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ConfigStatusDraft       = "DRAFT"
	ConfigStatusPublished   = "PUBLISHED"
	ConfigStatusRetired     = "RETIRED"
	StepStatusNotStarted    = "NOT_STARTED"
	StepStatusInProgress    = "IN_PROGRESS"
	StepStatusWaitingDoctor = "WAITING_DOCTOR"
	StepStatusBlocked       = "BLOCKED"
	StepStatusSkipped       = "SKIPPED"
)

type TreatmentProgram struct {
	BaseModel
	CompanyID        uuid.UUID       `json:"company_id" gorm:"type:uuid;not null;uniqueIndex:idx_program_company_code"`
	Code             string          `json:"code" gorm:"size:50;not null;uniqueIndex:idx_program_company_code"`
	Name             string          `json:"name" gorm:"size:180;not null"`
	Category         string          `json:"category" gorm:"size:30;not null;index"`
	Description      string          `json:"description" gorm:"type:text"`
	Status           string          `json:"status" gorm:"size:30;not null;index"`
	CurrentVersionID *uuid.UUID      `json:"current_version_id,omitempty" gorm:"type:uuid;index"`
	Company          *Company        `json:"company,omitempty" gorm:"foreignKey:CompanyID"`
	CurrentVersion   *ProgramVersion `json:"current_version,omitempty" gorm:"foreignKey:CurrentVersionID"`
}

type ProgramVersion struct {
	BaseModel
	ProgramID         uuid.UUID         `json:"program_id" gorm:"type:uuid;not null;uniqueIndex:idx_program_version"`
	VersionNumber     int               `json:"version_number" gorm:"not null;uniqueIndex:idx_program_version"`
	Status            string            `json:"status" gorm:"size:30;not null;index"`
	JourneyWorkflowID *uuid.UUID        `json:"journey_workflow_id,omitempty" gorm:"type:uuid;index"`
	ChangeNote        string            `json:"change_note" gorm:"type:text"`
	PublishedAt       *time.Time        `json:"published_at,omitempty"`
	Program           *TreatmentProgram `json:"program,omitempty" gorm:"foreignKey:ProgramID"`
	JourneyWorkflow   *WorkflowTemplate `json:"journey_workflow,omitempty" gorm:"foreignKey:JourneyWorkflowID"`
	StepTemplates     []StepTemplate    `json:"step_templates,omitempty" gorm:"foreignKey:ProgramVersionID"`
}

type StepTemplate struct {
	BaseModel
	ProgramVersionID   uuid.UUID            `json:"program_version_id" gorm:"type:uuid;not null;uniqueIndex:idx_step_template_code"`
	Code               string               `json:"code" gorm:"size:100;not null;uniqueIndex:idx_step_template_code"`
	Name               string               `json:"name" gorm:"size:180;not null"`
	StepType           string               `json:"step_type" gorm:"size:50;not null;index"`
	DisplayOrder       int                  `json:"display_order" gorm:"not null"`
	IsRequired         bool                 `json:"is_required" gorm:"not null;default:true"`
	AssignedRoleID     *uuid.UUID           `json:"assigned_role_id,omitempty" gorm:"type:uuid;index"`
	DueDays            *int                 `json:"due_days,omitempty"`
	CreatesOrder       bool                 `json:"creates_order" gorm:"not null;default:false"`
	RepeatType         string               `json:"repeat_type" gorm:"size:30;not null;default:NONE"`
	RepeatIntervalDays *int                 `json:"repeat_interval_days,omitempty"`
	MaxOccurrences     *int                 `json:"max_occurrences,omitempty"`
	Condition          datatypes.JSON       `json:"condition" gorm:"type:jsonb;not null;default:'{}'"`
	WorkflowTemplateID *uuid.UUID           `json:"workflow_template_id,omitempty" gorm:"type:uuid;index"`
	ProgramVersion     *ProgramVersion      `json:"program_version,omitempty" gorm:"foreignKey:ProgramVersionID"`
	AssignedRole       *Role                `json:"assigned_role,omitempty" gorm:"foreignKey:AssignedRoleID"`
	WorkflowTemplate   *WorkflowTemplate    `json:"workflow_template,omitempty" gorm:"foreignKey:WorkflowTemplateID"`
	Dependencies       []StepDependency     `json:"dependencies,omitempty" gorm:"foreignKey:StepTemplateID"`
	FormAssignments    []StepFormAssignment `json:"form_assignments,omitempty" gorm:"foreignKey:StepTemplateID"`
}

type StepDependency struct {
	ID              uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	StepTemplateID  uuid.UUID      `json:"step_template_id" gorm:"type:uuid;not null;uniqueIndex:idx_step_dependency"`
	DependsOnStepID uuid.UUID      `json:"depends_on_step_id" gorm:"type:uuid;not null;uniqueIndex:idx_step_dependency"`
	Condition       datatypes.JSON `json:"condition" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt       time.Time      `json:"created_at"`
	StepTemplate    *StepTemplate  `json:"step_template,omitempty" gorm:"foreignKey:StepTemplateID"`
	DependsOnStep   *StepTemplate  `json:"depends_on_step,omitempty" gorm:"foreignKey:DependsOnStepID"`
}

func (m *StepDependency) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

type FormTemplate struct {
	BaseModel
	CompanyID        uuid.UUID    `json:"company_id" gorm:"type:uuid;not null;uniqueIndex:idx_form_company_code"`
	Code             string       `json:"code" gorm:"size:100;not null;uniqueIndex:idx_form_company_code"`
	Name             string       `json:"name" gorm:"size:180;not null"`
	Description      string       `json:"description" gorm:"type:text"`
	Status           string       `json:"status" gorm:"size:30;not null;index"`
	CurrentVersionID *uuid.UUID   `json:"current_version_id,omitempty" gorm:"type:uuid;index"`
	CurrentVersion   *FormVersion `json:"current_version,omitempty" gorm:"foreignKey:CurrentVersionID"`
}

type FormVersion struct {
	BaseModel
	FormTemplateID uuid.UUID      `json:"form_template_id" gorm:"type:uuid;not null;uniqueIndex:idx_form_version"`
	VersionNumber  int            `json:"version_number" gorm:"not null;uniqueIndex:idx_form_version"`
	Status         string         `json:"status" gorm:"size:30;not null;index"`
	Schema         datatypes.JSON `json:"schema" gorm:"type:jsonb;not null"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	FormTemplate   *FormTemplate  `json:"form_template,omitempty" gorm:"foreignKey:FormTemplateID"`
}

type StepFormAssignment struct {
	ID             uuid.UUID     `json:"id" gorm:"type:uuid;primaryKey"`
	StepTemplateID uuid.UUID     `json:"step_template_id" gorm:"type:uuid;not null;uniqueIndex:idx_step_form_assignment"`
	FormVersionID  uuid.UUID     `json:"form_version_id" gorm:"type:uuid;not null;uniqueIndex:idx_step_form_assignment"`
	Phase          string        `json:"phase" gorm:"size:20;not null;uniqueIndex:idx_step_form_assignment"`
	ActorType      string        `json:"actor_type" gorm:"size:20;not null"`
	IsRequired     bool          `json:"is_required" gorm:"not null;default:true"`
	CreatedAt      time.Time     `json:"created_at"`
	StepTemplate   *StepTemplate `json:"step_template,omitempty" gorm:"foreignKey:StepTemplateID"`
	FormVersion    *FormVersion  `json:"form_version,omitempty" gorm:"foreignKey:FormVersionID"`
}

func (m *StepFormAssignment) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

type WorkflowTemplate struct {
	BaseModel
	CompanyID   uuid.UUID            `json:"company_id" gorm:"type:uuid;not null;uniqueIndex:idx_workflow_company_code"`
	Code        string               `json:"code" gorm:"size:100;not null;uniqueIndex:idx_workflow_company_code"`
	Name        string               `json:"name" gorm:"size:180;not null"`
	EntityType  string               `json:"entity_type" gorm:"size:20;not null;index"`
	Status      string               `json:"status" gorm:"size:30;not null;index"`
	States      []WorkflowState      `json:"states,omitempty" gorm:"foreignKey:WorkflowTemplateID"`
	Transitions []WorkflowTransition `json:"transitions,omitempty" gorm:"foreignKey:WorkflowTemplateID"`
}

type WorkflowState struct {
	BaseModel
	WorkflowTemplateID uuid.UUID `json:"workflow_template_id" gorm:"type:uuid;not null;uniqueIndex:idx_workflow_state_code"`
	Code               string    `json:"code" gorm:"size:50;not null;uniqueIndex:idx_workflow_state_code"`
	StandardCode       string    `json:"standard_code" gorm:"size:50;not null;index"`
	DisplayName        string    `json:"display_name" gorm:"size:100;not null"`
	DisplayOrder       int       `json:"display_order" gorm:"not null"`
	IsInitial          bool      `json:"is_initial" gorm:"not null;default:false"`
	IsTerminal         bool      `json:"is_terminal" gorm:"not null;default:false"`
}

type WorkflowTransition struct {
	BaseModel
	WorkflowTemplateID uuid.UUID  `json:"workflow_template_id" gorm:"type:uuid;not null;index"`
	FromStateCode      string     `json:"from_state_code" gorm:"size:50;not null;uniqueIndex:idx_workflow_transition"`
	ToStateCode        string     `json:"to_state_code" gorm:"size:50;not null;uniqueIndex:idx_workflow_transition"`
	RequiredRoleID     *uuid.UUID `json:"required_role_id,omitempty" gorm:"type:uuid;index"`
	RequiredRole       *Role      `json:"required_role,omitempty" gorm:"foreignKey:RequiredRoleID"`
}

type ProgramLocation struct {
	ProgramID  uuid.UUID         `json:"program_id" gorm:"type:uuid;primaryKey"`
	LocationID uuid.UUID         `json:"location_id" gorm:"type:uuid;primaryKey"`
	Status     string            `json:"status" gorm:"size:30;not null"`
	Config     datatypes.JSON    `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Program    *TreatmentProgram `json:"program,omitempty" gorm:"foreignKey:ProgramID"`
	Location   *Location         `json:"location,omitempty" gorm:"foreignKey:LocationID"`
}

type StatusHistory struct {
	ID                      uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	CompanyID               uuid.UUID `json:"company_id" gorm:"type:uuid;not null;index"`
	EntityType              string    `json:"entity_type" gorm:"size:20;not null;index"`
	EntityID                uuid.UUID `json:"entity_id" gorm:"type:uuid;not null;index"`
	FromStatus              string    `json:"from_status" gorm:"size:50;not null"`
	ToStatus                string    `json:"to_status" gorm:"size:50;not null"`
	PerformedByMembershipID uuid.UUID `json:"performed_by_membership_id" gorm:"type:uuid;not null;index"`
	Note                    string    `json:"note" gorm:"type:text"`
	CreatedAt               time.Time `json:"created_at" gorm:"index"`
}

func (m *StatusHistory) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
