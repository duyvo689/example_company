DROP INDEX IF EXISTS idx_orders_journey_step;
ALTER TABLE orders DROP COLUMN IF EXISTS journey_step_id;
DROP TABLE IF EXISTS patient_churn_events;
DROP TABLE IF EXISTS intake_files;
DROP TABLE IF EXISTS intake_submissions;
DROP TABLE IF EXISTS renewal_links;
DROP TABLE IF EXISTS journey_steps;
DROP TABLE IF EXISTS care_journeys;
