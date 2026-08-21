-- 0004_role_credential — coaching licenses and referee recertifications.
-- These are the "role credential" input to eligibility: roles that require a
-- credential (coach, referee) are ineligible when it is missing or expired,
-- exactly like SafeSport and the background check (domain-model.md section 4).

create table role_credential (
    id              uuid primary key default gen_random_uuid(),
    member_id       uuid not null references member(id) on delete cascade,
    -- Which role this credential satisfies. Must be a credential-requiring role.
    role            text not null check (role in ('coach','referee')),
    -- e.g. 'grassroots_license', 'd_license', 'referee_recert'
    credential_type text not null,
    issued_at       date not null,
    expires_at      date not null,
    created_at      timestamptz not null default now()
);
create index on role_credential (member_id);
