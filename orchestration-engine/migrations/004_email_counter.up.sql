-- Add email dispatch counter to workflow_runs table
ALTER TABLE workflow_runs ADD COLUMN email_dispatch_count INT DEFAULT 0;

-- Add index for better performance
CREATE INDEX idx_workflow_runs_email_count ON workflow_runs(execution_id, email_dispatch_count);
