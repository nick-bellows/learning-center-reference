-- Synthetic, fictional seed data for LOCAL demo/testing ONLY.
-- No real people. Do not add real member data to this file.
--
-- Fully idempotent: every insert has a fixed id + ON CONFLICT DO NOTHING, so
-- the API can apply it on every startup (SEED_FILE) without duplicating rows.
--
-- The learner, two referee examples, and administrator demonstrate role-based views:
--   Alex Coach    -> eligible          (everything current, incl. coaching license)
--   Sam Referee   -> suspended         (active hold overrides current credentials)
--   Riley Referee -> ineligible_lapsed (current checks, EXPIRED referee recert)
--   Casey Admin   -> eligible          (current universal safeguarding inputs)

insert into member_association (id, name, slug) values
  ('00000000-0000-0000-0000-0000000000aa', 'Sample Youth Soccer Association', 'sample-ysa')
on conflict do nothing;

insert into member (id, auth_subject, display_name, date_of_birth, association_id) values
  ('11111111-1111-1111-1111-111111111111', 'demo|learner', 'Alex Coach (synthetic)',    '1990-05-01', '00000000-0000-0000-0000-0000000000aa'),
  ('22222222-2222-2222-2222-222222222222', null,           'Sam Referee (synthetic)',   '1988-09-15', '00000000-0000-0000-0000-0000000000aa'),
  ('33333333-3333-3333-3333-333333333333', null,           'Riley Referee (synthetic)', '1995-03-20', '00000000-0000-0000-0000-0000000000aa'),
  ('44444444-4444-4444-4444-444444444444', 'demo|admin',   'Casey Admin (synthetic)',   '1986-11-04', '00000000-0000-0000-0000-0000000000aa')
on conflict (id) do update set auth_subject = excluded.auth_subject;

insert into member_role (member_id, role) values
  ('11111111-1111-1111-1111-111111111111', 'learner'),
  ('11111111-1111-1111-1111-111111111111', 'coach'),
  ('22222222-2222-2222-2222-222222222222', 'referee'),
  ('33333333-3333-3333-3333-333333333333', 'referee'),
  ('44444444-4444-4444-4444-444444444444', 'admin')
on conflict do nothing;

-- Background checks: all three current.
insert into background_check (id, member_id, source, approved_at, expires_at, status) values
  ('aaaa0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved'),
  ('aaaa0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved'),
  ('aaaa0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved'),
  ('aaaa0000-0000-0000-0000-000000000004', '44444444-4444-4444-4444-444444444444', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved')
on conflict do nothing;

-- SafeSport: all three current.
insert into safesport_training (id, member_id, training_type, completed_at, expires_at) values
  ('bbbb0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'core', '2026-06-01', '2027-06-01'),
  ('bbbb0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'core', '2026-06-01', '2027-06-01'),
  ('bbbb0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'core', '2026-06-01', '2027-06-01'),
  ('bbbb0000-0000-0000-0000-000000000004', '44444444-4444-4444-4444-444444444444', 'core', '2026-06-01', '2027-06-01')
on conflict do nothing;

-- Role credentials: Alex's coaching license and Sam's recert are current;
-- Riley's referee recertification EXPIRED — the lapse that flips eligibility.
insert into role_credential (id, member_id, role, credential_type, issued_at, expires_at) values
  ('cccc0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'coach',   'grassroots_license', '2025-02-01', '2027-02-01'),
  ('cccc0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'referee', 'referee_recert',     '2026-01-15', '2027-01-15'),
  ('cccc0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'referee', 'referee_recert',     '2024-01-15', '2025-01-15')
on conflict do nothing;

-- Sam's active hold (lifted_at NULL = active): overrides everything above.
insert into disciplinary_hold (id, member_id, source, reason) values
  ('dddd0000-0000-0000-0000-000000000001', '22222222-2222-2222-2222-222222222222', 'safesport', 'synthetic example hold')
on conflict do nothing;

-- One bounded course is enough to prove the learner workflow. Rich content would
-- remain in a headless CMS; this reference stores only non-secret content references.
insert into course (id, association_id, title, slug, ordering, status) values
  ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000aa',
   'Grassroots Match-Day Safety', 'grassroots-match-day-safety', 'sequential', 'published')
on conflict do nothing;

insert into module (id, course_id, title, position) values
  ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001',
   'Prepare, respond, document', 1)
on conflict do nothing;

insert into lesson (id, module_id, title, lesson_type, position, content_ref) values
  ('30000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'Pre-match risk check', 'reading', 1, 'synthetic.safety.pre-match'),
  ('30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', 'Incident response', 'video', 2, 'synthetic.safety.response'),
  ('30000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000001', 'Knowledge check', 'quiz', 3, 'synthetic.safety.assessment')
on conflict do nothing;

insert into assessment (id, lesson_id, passing_threshold, max_attempts) values
  ('40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000003', 80, 3)
on conflict do nothing;
