-- 0003_safeguarding — THE MOAT.
-- These are the independently-expiring INPUTS to a member's participation eligibility.
-- Eligibility itself is NOT stored here. It is COMPUTED in the Go domain layer (and
-- unit-tested) from these rows, per docs/domain-model.md section 4. Storing a derived
-- "eligible" flag would go stale the instant any of these expired.

-- Background check — issued by the local club/association/org; valid ~2 years.
create table background_check (
    id          uuid primary key default gen_random_uuid(),
    member_id   uuid not null references member(id) on delete cascade,
    source      text not null,                    -- who issued/verified it
    approved_at date not null,
    expires_at  date not null,                    -- explicit date; don't hardcode a season end
    status      text not null default 'approved' check (status in ('pending','approved','rejected')),
    created_at  timestamptz not null default now()
);
create index on background_check (member_id);

-- SafeSport — "core" then annual "refresher". Effectively an annually-expiring credential.
create table safesport_training (
    id            uuid primary key default gen_random_uuid(),
    member_id     uuid not null references member(id) on delete cascade,
    training_type text not null check (training_type in ('core','refresher')),
    completed_at  date not null,
    expires_at    date not null,                  -- typically 1 year after completion
    created_at    timestamptz not null default now()
);
create index on safesport_training (member_id);

-- Disciplinary hold — Nick's Q1: a flag can come from U.S. Soccer, SafeSport, or a local
-- org depending on where the report originates, and it makes a member ineligible
-- REGARDLESS of their training/check status.
create table disciplinary_hold (
    id         uuid primary key default gen_random_uuid(),
    member_id  uuid not null references member(id) on delete cascade,
    source     text not null check (source in ('us_soccer','safesport','local_org')),
    reason     text,                              -- kept minimal; no sensitive detail stored
    placed_at  timestamptz not null default now(),
    lifted_at  timestamptz,                       -- NULL = the hold is still ACTIVE
    created_at timestamptz not null default now()
);
-- Partial index: index only the ACTIVE holds (the ones eligibility checks care about).
create index on disciplinary_hold (member_id) where lifted_at is null;
