CREATE TABLE workflow_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    code varchar(100) NOT NULL, name varchar(180) NOT NULL,
    entity_type varchar(20) NOT NULL CHECK (entity_type IN ('JOURNEY', 'STEP')),
    status varchar(30) NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'PUBLISHED', 'RETIRED')),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (company_id, code)
);

CREATE TABLE workflow_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workflow_template_id uuid NOT NULL REFERENCES workflow_templates(id),
    code varchar(50) NOT NULL, standard_code varchar(50) NOT NULL, display_name varchar(100) NOT NULL,
    display_order integer NOT NULL, is_initial boolean NOT NULL DEFAULT false, is_terminal boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (workflow_template_id, code)
);
CREATE UNIQUE INDEX one_initial_state_per_workflow ON workflow_states(workflow_template_id) WHERE is_initial AND deleted_at IS NULL;

CREATE TABLE workflow_transitions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workflow_template_id uuid NOT NULL REFERENCES workflow_templates(id),
    from_state_code varchar(50) NOT NULL, to_state_code varchar(50) NOT NULL,
    required_role_id uuid REFERENCES roles(id),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (workflow_template_id, from_state_code, to_state_code),
    CHECK (from_state_code <> to_state_code)
);

CREATE TABLE treatment_programs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    code varchar(50) NOT NULL, name varchar(180) NOT NULL,
    category varchar(30) NOT NULL CHECK (category IN ('DENTAL', 'AESTHETIC', 'GENERAL')),
    description text NOT NULL DEFAULT '', status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    current_version_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (company_id, code)
);

CREATE TABLE program_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), program_id uuid NOT NULL REFERENCES treatment_programs(id),
    version_number integer NOT NULL CHECK (version_number > 0),
    status varchar(30) NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'PUBLISHED', 'RETIRED')),
    journey_workflow_id uuid REFERENCES workflow_templates(id), change_note text NOT NULL DEFAULT '', published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (program_id, version_number)
);
CREATE UNIQUE INDEX one_published_version_per_program ON program_versions(program_id) WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
ALTER TABLE treatment_programs ADD CONSTRAINT fk_program_current_version FOREIGN KEY (current_version_id) REFERENCES program_versions(id);

CREATE TABLE program_locations (
    program_id uuid NOT NULL REFERENCES treatment_programs(id), location_id uuid NOT NULL REFERENCES locations(id),
    status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    config jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (program_id, location_id)
);

CREATE TABLE step_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), program_version_id uuid NOT NULL REFERENCES program_versions(id),
    code varchar(100) NOT NULL, name varchar(180) NOT NULL, step_type varchar(50) NOT NULL,
    display_order integer NOT NULL, is_required boolean NOT NULL DEFAULT true,
    assigned_role_id uuid REFERENCES roles(id), due_days integer CHECK (due_days >= 0),
    creates_order boolean NOT NULL DEFAULT false,
    repeat_type varchar(30) NOT NULL DEFAULT 'NONE' CHECK (repeat_type IN ('NONE', 'INTERVAL')),
    repeat_interval_days integer CHECK (repeat_interval_days > 0), max_occurrences integer CHECK (max_occurrences > 0),
    condition jsonb NOT NULL DEFAULT '{}'::jsonb, workflow_template_id uuid REFERENCES workflow_templates(id),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (program_version_id, code)
);

CREATE TABLE step_dependencies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), step_template_id uuid NOT NULL REFERENCES step_templates(id),
    depends_on_step_id uuid NOT NULL REFERENCES step_templates(id), condition jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (step_template_id, depends_on_step_id),
    CHECK (step_template_id <> depends_on_step_id)
);

CREATE TABLE form_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    code varchar(100) NOT NULL, name varchar(180) NOT NULL, description text NOT NULL DEFAULT '',
    status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')), current_version_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (company_id, code)
);

CREATE TABLE form_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), form_template_id uuid NOT NULL REFERENCES form_templates(id),
    version_number integer NOT NULL CHECK (version_number > 0),
    status varchar(30) NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'PUBLISHED', 'RETIRED')),
    schema jsonb NOT NULL, published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (form_template_id, version_number)
);
CREATE UNIQUE INDEX one_published_version_per_form ON form_versions(form_template_id) WHERE status = 'PUBLISHED' AND deleted_at IS NULL;
ALTER TABLE form_templates ADD CONSTRAINT fk_form_current_version FOREIGN KEY (current_version_id) REFERENCES form_versions(id);

CREATE TABLE step_form_assignments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), step_template_id uuid NOT NULL REFERENCES step_templates(id),
    form_version_id uuid NOT NULL REFERENCES form_versions(id),
    phase varchar(20) NOT NULL CHECK (phase IN ('BEFORE', 'DURING', 'AFTER')),
    actor_type varchar(20) NOT NULL CHECK (actor_type IN ('PATIENT', 'STAFF', 'DOCTOR')),
    is_required boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (step_template_id, form_version_id, phase)
);

CREATE TABLE status_histories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    entity_type varchar(20) NOT NULL CHECK (entity_type IN ('JOURNEY', 'STEP')),
    entity_id uuid NOT NULL, from_status varchar(50) NOT NULL, to_status varchar(50) NOT NULL,
    performed_by_membership_id uuid NOT NULL REFERENCES memberships(id), note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_status_history_entity ON status_histories(entity_type, entity_id, created_at DESC);

ALTER TABLE care_journeys
    ADD COLUMN program_version_id uuid REFERENCES program_versions(id),
    ADD COLUMN context_data jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE care_journeys DROP CONSTRAINT IF EXISTS care_journeys_status_check;
ALTER TABLE care_journeys ADD CONSTRAINT care_journeys_status_check
    CHECK (status IN ('DRAFT', 'ONBOARDING', 'ACTIVE', 'PAUSED', 'COMPLETED', 'CHURNED', 'CANCELLED'));

ALTER TABLE journey_steps
    ADD COLUMN step_template_id uuid REFERENCES step_templates(id),
    ADD COLUMN template_iteration integer NOT NULL DEFAULT 1 CHECK (template_iteration > 0);

ALTER TABLE journey_steps DROP CONSTRAINT IF EXISTS journey_steps_status_check;
ALTER TABLE journey_steps ADD CONSTRAINT journey_steps_status_check CHECK (status IN (
    'PENDING', 'NOT_STARTED', 'READY', 'WAITING_PATIENT', 'IN_PROGRESS', 'IN_REVIEW',
    'WAITING_DOCTOR', 'BLOCKED', 'SUBMITTED', 'COMPLETED', 'SKIPPED', 'EXPIRED', 'REJECTED', 'CANCELLED'
));

ALTER TABLE intake_submissions ADD COLUMN form_version_id uuid REFERENCES form_versions(id);
CREATE INDEX idx_journeys_program_version ON care_journeys(program_version_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_steps_template ON journey_steps(step_template_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_submissions_form_version ON intake_submissions(form_version_id) WHERE deleted_at IS NULL;
