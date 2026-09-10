-- Reverses the grants. The role itself is left in place: roles are cluster-wide and a
-- hosted deployment may have granted it to a login role, which would make DROP ROLE fail.
-- Drop it by hand once nothing depends on it: `drop role lcr_runtime;`
alter default privileges in schema public
    revoke select, insert, update, delete on tables from lcr_runtime;
revoke all privileges on all tables in schema public from lcr_runtime;
revoke usage on schema public from lcr_runtime;
