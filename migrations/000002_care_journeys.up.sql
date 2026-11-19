CREATE TABLE care_journeys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    patient_id uuid NOT NULL REFERENCES patients(id),
    location_id uuid NOT NULL REFERENCES locations(id),
    created_by_membership_id uuid NOT NULL REFERENCES memberships(id),
    program_code varchar(50) NOT NULL,
    episode_number integer NOT NULL CHECK (episode_number > 0),
    status varchar(30) NOT NULL CHECK (status IN ('ONBOARDING', 'ACTIVE', 'PAUSED', 'COMPLETED', 'CHURNED', 'CANCELLED')),
    current_stage varchar(50) NOT NULL,
    current_cycle integer NOT NULL DEFAULT 1 CHECK (current_cycle > 0),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (company_id, patient_id, program_code, episode_number)
);

CREATE UNIQUE INDEX one_active_journey_per_program
    ON care_journeys(company_id, patient_id, program_code)
    WHERE status IN ('ONBOARDING', 'ACTIVE', 'PAUSED') AND deleted_at IS NULL;
CREATE INDEX idx_care_journeys_company_created ON care_journeys(company_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_care_journeys_location ON care_journeys(location_id) WHERE deleted_at IS NULL;

CREATE TABLE journey_steps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    journey_id uuid NOT NULL REFERENCES care_journeys(id),
    assigned_membership_id uuid REFERENCES memberships(id),
    step_key varchar(100) NOT NULL,
    step_type varchar(50) NOT NULL,
    cycle_number integer NOT NULL DEFAULT 1 CHECK (cycle_number > 0),
    status varchar(30) NOT NULL CHECK (status IN ('PENDING', 'WAITING_PATIENT', 'IN_REVIEW', 'READY', 'SUBMITTED', 'COMPLETED', 'EXPIRED', 'REJECTED', 'CANCELLED')),
    due_at timestamptz,
    completed_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (journey_id, step_key)
);

CREATE INDEX idx_journey_steps_company_status ON journey_steps(company_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_journey_steps_assignee ON journey_steps(assigned_membership_id) WHERE deleted_at IS NULL;

CREATE TABLE renewal_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    journey_step_id uuid NOT NULL REFERENCES journey_steps(id),
    token_hash varchar(64) NOT NULL UNIQUE,
    verification_hash varchar(100) NOT NULL,
    status varchar(30) NOT NULL CHECK (status IN ('ISSUED', 'OPENED', 'SUBMITTED', 'COMPLETED', 'EXPIRED', 'REVOKED', 'LOCKED')),
    attempt_count integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 5 CHECK (max_attempts BETWEEN 1 AND 10),
    expires_at timestamptz NOT NULL,
    opened_at timestamptz,
    submitted_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_renewal_links_step_status ON renewal_links(journey_step_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_renewal_links_expiry ON renewal_links(status, expires_at) WHERE deleted_at IS NULL;

CREATE TABLE intake_submissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    patient_id uuid REFERENCES patients(id),
    location_id uuid NOT NULL REFERENCES locations(id),
    journey_id uuid REFERENCES care_journeys(id),
    journey_step_id uuid REFERENCES journey_steps(id),
    renewal_link_id uuid REFERENCES renewal_links(id),
    intake_type varchar(50) NOT NULL CHECK (intake_type IN ('NEW_PATIENT_INTAKE', 'TRT_INTAKE', 'GLP1_INTAKE', 'TRT_RENEWAL', 'GLP1_RENEWAL', 'FOLLOW_UP', 'ADDRESS_UPDATE')),
    schema_version integer NOT NULL CHECK (schema_version > 0),
    status varchar(30) NOT NULL CHECK (status IN ('SUBMITTED', 'VALIDATED', 'REVIEWED', 'REJECTED')),
    answers jsonb NOT NULL,
    clinical_flags jsonb NOT NULL DEFAULT '{}'::jsonb,
    submitted_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX one_submission_per_renewal
    ON intake_submissions(renewal_link_id)
    WHERE renewal_link_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_intake_company_submitted ON intake_submissions(company_id, submitted_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_intake_journey ON intake_submissions(journey_id) WHERE deleted_at IS NULL;

CREATE TABLE intake_files (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    submission_id uuid NOT NULL REFERENCES intake_submissions(id),
    file_type varchar(50) NOT NULL,
    storage_key varchar(500) NOT NULL UNIQUE,
    mime_type varchar(100) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_intake_files_submission ON intake_files(submission_id) WHERE deleted_at IS NULL;

CREATE TABLE patient_churn_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    journey_id uuid NOT NULL REFERENCES care_journeys(id),
    patient_id uuid NOT NULL REFERENCES patients(id),
    location_id uuid NOT NULL REFERENCES locations(id),
    performed_by_membership_id uuid NOT NULL REFERENCES memberships(id),
    event_type varchar(30) NOT NULL CHECK (event_type IN ('CHURNED', 'REACTIVATED', 'TRANSFERRED', 'COMPLETED', 'CLOSED')),
    stage_at_event varchar(50) NOT NULL,
    reason_code varchar(50) NOT NULL,
    note text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_churn_journey_occurred ON patient_churn_events(journey_id, occurred_at DESC);

ALTER TABLE orders
    ADD COLUMN journey_step_id uuid REFERENCES journey_steps(id);
CREATE INDEX idx_orders_journey_step ON orders(journey_step_id) WHERE deleted_at IS NULL;
