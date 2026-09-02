-- 0005_progress — a deliberately bounded event-sourcing example.
-- Lesson completions are immutable facts; enrollment_progress is the read projection.
-- Both are written in one transaction by the Go store, so readers never see drift.

create table progress_event (
    id              uuid primary key default gen_random_uuid(),
    enrollment_id   uuid not null references enrollment(id) on delete cascade,
    lesson_id       uuid not null references lesson(id) on delete cascade,
    actor_member_id uuid not null references member(id) on delete restrict,
    event_type      text not null check (event_type in ('lesson_completed')),
    occurred_at     timestamptz not null default now(),
    unique (enrollment_id, lesson_id, event_type)
);
create index progress_event_enrollment_time_idx
    on progress_event (enrollment_id, occurred_at);

create table enrollment_progress (
    enrollment_id     uuid primary key references enrollment(id) on delete cascade,
    completed_lessons int not null default 0 check (completed_lessons >= 0),
    total_lessons     int not null check (total_lessons >= 0),
    percent_complete  int not null default 0 check (percent_complete between 0 and 100),
    updated_at        timestamptz not null default now(),
    check (completed_lessons <= total_lessons)
);

comment on table progress_event is
    'Append-only learner progress facts; application roles may insert but never update/delete.';
comment on table enrollment_progress is
    'Transactional read projection derived from progress_event for learner dashboards.';

-- Backfill any enrollment created before this migration. There are no historical
-- completion events to infer, so each starts at zero with an accurate lesson count.
insert into enrollment_progress (enrollment_id, total_lessons)
select e.id, count(l.id)::int
from enrollment e
left join module m on m.course_id = e.course_id
left join lesson l on l.module_id = m.id
group by e.id
on conflict (enrollment_id) do nothing;
