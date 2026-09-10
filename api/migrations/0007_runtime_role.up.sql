-- 0007_runtime_role — a least-privilege role for the running API.
--
-- Migrations and the synthetic seed run as the database owner (the connection in
-- DATABASE_URL). Request handling should not: a compromised handler or an injection bug
-- must not be able to alter the schema, rewrite history in the append-only progress log,
-- or read the migration ledger. This role holds exactly the DML the store uses.
--
-- lcr_runtime cannot log in. The API adopts it in one of two ways (see cmd/server):
--   * DB_RUNTIME_ROLE=lcr_runtime — every pooled connection runs SET ROLE after
--     connecting (the local Compose default; no second password to manage), or
--   * RUNTIME_DATABASE_URL — a separate login role that is a member of lcr_runtime
--     (the shape for a hosted deployment; the login role inherits these privileges).
--
-- Roles are cluster-wide, so creation tolerates a concurrent creator (another database
-- on the same server applying this migration at the same time).

do $$
begin
    create role lcr_runtime nologin;
exception
    when duplicate_object then
        raise notice 'role lcr_runtime already exists';
end $$;

grant usage on schema public to lcr_runtime;
grant select, insert, update, delete on all tables in schema public to lcr_runtime;

-- The migration ledger is the owner's business only.
revoke all privileges on schema_migrations from lcr_runtime;

-- progress_event is append-only by design (see 0005). Enforce it with privileges, not
-- just convention: the runtime role may add facts but never change or remove them.
revoke update, delete on progress_event from lcr_runtime;

-- Tables created by later migrations (run by the same owner) get the same DML grant.
alter default privileges in schema public
    grant select, insert, update, delete on tables to lcr_runtime;

comment on role lcr_runtime is
    'DML-only role adopted by the Learning Center API for request handling; cannot log in, change schema, or modify progress_event history.';
