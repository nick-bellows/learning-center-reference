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
    -- The identity provider's "sub" (subject) claim links this row to the external IdP.
    -- Nullable so synthetic members who never sign in can be seeded.
    auth_subject   text unique,
    display_name   text not null,
    -- date_of_birth is stored; minor/adult status is derived at query time rather than
    -- stored as an age, which would go stale the moment a birthday passes.
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
