-- 0001_identity — core identity: the people and the organizations they belong to.
-- Run order matters: later migrations reference these tables.

-- A member association = a state association / club / org that scopes courses and members.
create table member_association (
    id         uuid primary key default gen_random_uuid(),
    name       text not null,
    slug       text not null unique,
    created_at timestamptz not null default now()
);

-- A person in the system. One row per human.
create table member (
    id             uuid primary key default gen_random_uuid(),
    -- Auth0's "sub" (subject) claim. Links this row to the external identity provider.
    -- Nullable so we can seed synthetic members who never actually log in.
    auth_subject   text unique,
    display_name   text not null,
    -- We store date_of_birth and DERIVE "is this person a minor?" at query time.
    -- Never store the derived age — it goes stale the moment a birthday passes.
    date_of_birth  date,
    association_id uuid references member_association(id) on delete set null,
    created_at     timestamptz not null default now()
);

-- One person can hold several roles at once (a coach who also referees).
-- Composite primary key = a member can't have the same role twice.
create table member_role (
    member_id uuid not null references member(id) on delete cascade,
    role      text not null check (role in ('learner','instructor','admin','coach','referee')),
    primary key (member_id, role)
);
