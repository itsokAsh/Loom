-- Run once in Render Postgres shell (connect to trigger_db), before orchestration/worker start.
-- Render dashboard → loom-postgres → Connect → PSQL

CREATE DATABASE orchestration_db;
CREATE DATABASE worker_db;
