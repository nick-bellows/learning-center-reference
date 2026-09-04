-- 0006_chronology — a credential or check cannot expire before it begins. These guards stop
-- an internally inconsistent record (expiry earlier than issuance) from silently granting
-- eligibility. Effective-date filtering in the store handles the future-dated case; this
-- enforces the ordering at the schema level.

alter table background_check
    add constraint background_check_dates_ordered check (expires_at >= approved_at);
alter table safesport_training
    add constraint safesport_training_dates_ordered check (expires_at >= completed_at);
alter table role_credential
    add constraint role_credential_dates_ordered check (expires_at >= issued_at);
