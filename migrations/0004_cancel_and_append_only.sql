-- A run a person cancelled. Its checkpoint can no longer be replaced, which is
-- what stops the run: see Checkpoints.Save.
ALTER TABLE run_states DROP CONSTRAINT run_states_status_check;
ALTER TABLE run_states ADD CONSTRAINT run_states_status_check
    CHECK (status IN ('running', 'awaiting_approval', 'succeeded', 'failed', 'cancelled'));

-- The event record and the spend record are append-only. The database
-- enforces it, so no code path, old or new, can rewrite history.
CREATE FUNCTION merlin_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
END
$$;

CREATE TRIGGER events_no_change BEFORE UPDATE OR DELETE ON events
    FOR EACH ROW EXECUTE FUNCTION merlin_append_only();
CREATE TRIGGER events_no_truncate BEFORE TRUNCATE ON events
    FOR EACH STATEMENT EXECUTE FUNCTION merlin_append_only();

CREATE TRIGGER budget_entries_no_change BEFORE UPDATE OR DELETE ON budget_entries
    FOR EACH ROW EXECUTE FUNCTION merlin_append_only();
CREATE TRIGGER budget_entries_no_truncate BEFORE TRUNCATE ON budget_entries
    FOR EACH STATEMENT EXECUTE FUNCTION merlin_append_only();
