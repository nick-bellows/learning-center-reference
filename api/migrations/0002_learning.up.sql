-- 0002_learning — course content and enrollment.
-- Course -> Module -> Lesson -> (optional) Assessment. Members enroll in courses.

create table course (
    id             uuid primary key default gen_random_uuid(),
    association_id uuid references member_association(id) on delete set null,
    title          text not null,
    slug           text not null unique,
    -- Nick's Q4: "depends on the course, usually in order."
    -- 'sequential' = lessons must be done in order; 'open' = any order.
    ordering       text not null default 'sequential' check (ordering in ('sequential','open')),
    status         text not null default 'draft'      check (status in ('draft','published','archived')),
    created_at     timestamptz not null default now()
);

create table module (
    id        uuid primary key default gen_random_uuid(),
    course_id uuid not null references course(id) on delete cascade,
    title     text not null,
    position  int  not null,                       -- order within the course (1, 2, 3, ...)
    unique (course_id, position)                   -- no two modules share a slot in a course
);

create table lesson (
    id          uuid primary key default gen_random_uuid(),
    module_id   uuid not null references module(id) on delete cascade,
    title       text not null,
    lesson_type text not null check (lesson_type in ('video','reading','quiz')),
    position    int  not null,
    -- Rich lesson content lives in Sanity (headless CMS). We store only its reference.
    content_ref text,
    unique (module_id, position)
);

create table assessment (
    id                uuid primary key default gen_random_uuid(),
    -- one assessment per lesson (unique) — an assessment can't exist without its lesson.
    lesson_id         uuid not null unique references lesson(id) on delete cascade,
    passing_threshold int  not null check (passing_threshold between 0 and 100),
    max_attempts      int  check (max_attempts is null or max_attempts > 0)  -- null = unlimited
);

create table enrollment (
    id          uuid primary key default gen_random_uuid(),
    member_id   uuid not null references member(id) on delete cascade,
    course_id   uuid not null references course(id) on delete cascade,
    status      text not null default 'active' check (status in ('active','completed','withdrawn')),
    enrolled_at timestamptz not null default now(),
    unique (member_id, course_id)                  -- a member enrolls in a course at most once
);
