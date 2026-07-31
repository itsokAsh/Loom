-- Cascade deletes when removing a workflow from the UI
ALTER TABLE webhooks DROP CONSTRAINT IF EXISTS webhooks_workflow_id_fkey;
ALTER TABLE webhooks ADD CONSTRAINT webhooks_workflow_id_fkey
  FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE;

ALTER TABLE schedules DROP CONSTRAINT IF EXISTS schedules_workflow_id_fkey;
ALTER TABLE schedules ADD CONSTRAINT schedules_workflow_id_fkey
  FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE;

ALTER TABLE executions DROP CONSTRAINT IF EXISTS executions_workflow_id_fkey;
ALTER TABLE executions ADD CONSTRAINT executions_workflow_id_fkey
  FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE;

ALTER TABLE workflow_versions DROP CONSTRAINT IF EXISTS workflow_versions_workflow_id_fkey;
ALTER TABLE workflow_versions ADD CONSTRAINT workflow_versions_workflow_id_fkey
  FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE;
