-- Synthetic, fictional seed data for LOCAL demo/testing ONLY.
-- No real people. Do not add real member data to this file.
--
-- Fully idempotent: every insert has a fixed id + ON CONFLICT DO NOTHING, so
-- the API can apply it on every startup (SEED_FILE) without duplicating rows.
--
-- The three members demonstrate all three statuses:
--   Alex Coach    -> eligible          (everything current, incl. coaching license)
--   Sam Referee   -> suspended         (active hold overrides current credentials)
--   Riley Referee -> ineligible_lapsed (current checks, EXPIRED referee recert)

insert into member_association (id, name, slug) values
  ('00000000-0000-0000-0000-0000000000aa', 'Sample Youth Soccer Association', 'sample-ysa')
on conflict do nothing;

insert into member (id, display_name, date_of_birth, association_id) values
  ('11111111-1111-1111-1111-111111111111', 'Alex Coach (synthetic)',    '1990-05-01', '00000000-0000-0000-0000-0000000000aa'),
  ('22222222-2222-2222-2222-222222222222', 'Sam Referee (synthetic)',   '1988-09-15', '00000000-0000-0000-0000-0000000000aa'),
  ('33333333-3333-3333-3333-333333333333', 'Riley Referee (synthetic)', '1995-03-20', '00000000-0000-0000-0000-0000000000aa')
on conflict do nothing;

insert into member_role (member_id, role) values
  ('11111111-1111-1111-1111-111111111111', 'coach'),
  ('22222222-2222-2222-2222-222222222222', 'referee'),
  ('33333333-3333-3333-3333-333333333333', 'referee')
on conflict do nothing;

-- Background checks: all three current.
insert into background_check (id, member_id, source, approved_at, expires_at, status) values
  ('aaaa0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved'),
  ('aaaa0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved'),
  ('aaaa0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved')
on conflict do nothing;

-- SafeSport: all three current.
insert into safesport_training (id, member_id, training_type, completed_at, expires_at) values
  ('bbbb0000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'core', '2026-06-01', '2027-06-01'),
  ('bbbb0000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 'core', '2026-06-01', '2027-06-01'),
  ('bbbb0000-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'core', '2026-06-01', '2027-06-01')
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
