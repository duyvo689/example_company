CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE companies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name varchar(180) NOT NULL,
    status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email varchar(254) NOT NULL UNIQUE,
    name varchar(180) NOT NULL, password_hash varchar(100) NOT NULL,
    status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz
);

CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), code varchar(50) NOT NULL UNIQUE,
    scope varchar(30) NOT NULL CHECK (scope IN ('company', 'location')), description varchar(255) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz
);

CREATE TABLE locations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    code varchar(50) NOT NULL, name varchar(180) NOT NULL,
    status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (company_id, code)
);

CREATE TABLE memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    user_id uuid NOT NULL REFERENCES users(id), role_id uuid NOT NULL REFERENCES roles(id),
    status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'pending')),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (company_id, user_id)
);

CREATE TABLE patients (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    primary_location_id uuid REFERENCES locations(id), public_ref varchar(100) NOT NULL,
    external_chart_id varchar(100) NOT NULL DEFAULT '', status varchar(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (company_id, public_ref)
);

CREATE TABLE patient_locations (
    patient_id uuid NOT NULL REFERENCES patients(id), location_id uuid NOT NULL REFERENCES locations(id),
    relationship varchar(50) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (patient_id, location_id)
);

CREATE TABLE location_assignments (
    membership_id uuid NOT NULL REFERENCES memberships(id), location_id uuid NOT NULL REFERENCES locations(id),
    created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (membership_id, location_id)
);

CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    location_id uuid NOT NULL REFERENCES locations(id), patient_id uuid NOT NULL REFERENCES patients(id),
    created_by_membership_id uuid NOT NULL REFERENCES memberships(id), external_order_id varchar(100) NOT NULL,
    status varchar(30) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'cancelled', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
    UNIQUE (company_id, external_order_id)
);

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid REFERENCES companies(id),
    membership_id uuid REFERENCES memberships(id), request_id varchar(64) NOT NULL DEFAULT '',
    method varchar(10) NOT NULL, path varchar(500) NOT NULL, action varchar(50) NOT NULL,
    status_code integer NOT NULL, ip_address varchar(64) NOT NULL DEFAULT '', user_agent varchar(500) NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE integration_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    aggregate_type varchar(50) NOT NULL, aggregate_id uuid NOT NULL, event_type varchar(100) NOT NULL,
    payload jsonb NOT NULL, status varchar(30) NOT NULL DEFAULT 'pending', attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '', next_attempt_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id),
    token_hash varchar(64) NOT NULL UNIQUE, expires_at timestamptz NOT NULL, revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_companies_status ON companies(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_locations_company ON locations(company_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_memberships_company ON memberships(company_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_memberships_user ON memberships(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_company ON patients(company_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_orders_company_created ON orders(company_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_orders_location ON orders(location_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_audit_company_created ON audit_logs(company_id, created_at DESC);
CREATE INDEX idx_events_dispatch ON integration_events(status, next_attempt_at);
CREATE INDEX idx_refresh_expiry ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;

INSERT INTO roles (code, scope, description) VALUES
    ('ADMIN', 'company', 'Company administrator'),
    ('STAFF', 'company', 'Company staff'),
    ('CLINICIAN', 'location', 'Location-scoped clinician')
ON CONFLICT (code) DO NOTHING;
