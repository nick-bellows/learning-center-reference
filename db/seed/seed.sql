-- Synthetic, fictional seed data for LOCAL demo/testing ONLY.
-- No real people. Do not add real member data to this file.
--
-- Idempotent: every insert has a fixed id and an ON CONFLICT clause, so the API can apply
-- it on every startup (SEED_FILE) without duplicating rows. The member row uses DO UPDATE
-- so a re-seed refreshes auth_subject; every other row uses DO NOTHING.
--
-- Credential dates are relative to the moment of the first seed (current_date +/- an
-- interval), not hard-coded calendar dates, so the derived-eligibility demo shows the same
-- statuses whenever the project is cloned instead of silently lapsing on a fixed date.
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
  ('11111111-1111-1111-1111-111111111111', 'demo|learner',       'Alex Coach (synthetic)',    '1990-05-01', '00000000-0000-0000-0000-0000000000aa'),
  ('22222222-2222-2222-2222-222222222222', 'demo|referee-sam',   'Sam Referee (synthetic)',   '1988-09-15', '00000000-0000-0000-0000-0000000000aa'),
  ('33333333-3333-3333-3333-333333333333', 'demo|referee-riley', 'Riley Referee (synthetic)', '1995-03-20', '00000000-0000-0000-0000-0000000000aa'),
  ('44444444-4444-4444-4444-444444444444', 'demo|admin',         'Casey Admin (synthetic)',   '1986-11-04', '00000000-0000-0000-0000-0000000000aa')
on conflict (id) do update set auth_subject = excluded.auth_subject;

insert into member_role (member_id, role) values
  ('11111111-1111-1111-1111-111111111111', 'learner'),
  ('11111111-1111-1111-1111-111111111111', 'coach'),
  ('22222222-2222-2222-2222-222222222222', 'referee'),
  ('33333333-3333-3333-3333-333333333333', 'referee'),
  ('44444444-4444-4444-4444-444444444444', 'admin')
on conflict do nothing;

-- Background checks: all four current (valid ~2 years).
insert into background_check (id, member_id, source, approved_at, expires_at, status) values
  ('aaaa0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'sample-ysa', current_date - interval '3 months', (current_date + interval '2 years')::date, 'approved'),
  ('aaaa0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'sample-ysa', current_date - interval '3 months', (current_date + interval '2 years')::date, 'approved'),
  ('aaaa0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'sample-ysa', current_date - interval '3 months', (current_date + interval '2 years')::date, 'approved'),
  ('aaaa0000-0000-0000-0000-000000000004', '44444444-4444-4444-4444-444444444444', 'sample-ysa', current_date - interval '3 months', (current_date + interval '2 years')::date, 'approved')
on conflict do nothing;

-- SafeSport: all four current (annual refresher).
insert into safesport_training (id, member_id, training_type, completed_at, expires_at) values
  ('bbbb0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'core', current_date - interval '2 months', (current_date + interval '10 months')::date),
  ('bbbb0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'core', current_date - interval '2 months', (current_date + interval '10 months')::date),
  ('bbbb0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'core', current_date - interval '2 months', (current_date + interval '10 months')::date),
  ('bbbb0000-0000-0000-0000-000000000004', '44444444-4444-4444-4444-444444444444', 'core', current_date - interval '2 months', (current_date + interval '10 months')::date)
on conflict do nothing;

-- Role credentials: Alex's coaching license and Sam's recert are current; Riley's referee
-- recertification is EXPIRED — the lapse that flips Riley's eligibility to ineligible_lapsed.
insert into role_credential (id, member_id, role, credential_type, issued_at, expires_at) values
  ('cccc0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'coach',   'grassroots_license', current_date - interval '1 year',  (current_date + interval '1 year')::date),
  ('cccc0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'referee', 'referee_recert',     current_date - interval '6 months', (current_date + interval '6 months')::date),
  ('cccc0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'referee', 'referee_recert',     current_date - interval '18 months', (current_date - interval '8 months')::date)
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
