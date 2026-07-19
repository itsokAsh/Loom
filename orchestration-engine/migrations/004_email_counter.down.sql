-- Remove email dispatch counter from workflow_runs table
DROP INDEX IF EXISTS idx_workflow_runs_email_count;
ALTER TABLE workflow_runs DROP COLUMN IF EXISTS email_dispatch_count;
