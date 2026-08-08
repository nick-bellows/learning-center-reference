-- Synthetic, fictional seed data for LOCAL demo/testing ONLY.
-- No real people. Do not add real member data to this file.

insert into member_association (id, name, slug) values
  ('00000000-0000-0000-0000-0000000000aa', 'Sample Youth Soccer Association', 'sample-ysa')
on conflict do nothing;

insert into member (id, display_name, date_of_birth, association_id) values
  ('11111111-1111-1111-1111-111111111111', 'Alex Coach (synthetic)',    '1990-05-01', '00000000-0000-0000-0000-0000000000aa'),
  ('22222222-2222-2222-2222-222222222222', 'Sam Referee (synthetic)',   '1988-09-15', '00000000-0000-0000-0000-0000000000aa')
on conflict do nothing;

insert into member_role (member_id, role) values
  ('11111111-1111-1111-1111-111111111111', 'coach'),
  ('22222222-2222-2222-2222-222222222222', 'referee')
on conflict do nothing;

-- Member #1 (eligible): current background check + current SafeSport, no hold.
insert into background_check (member_id, source, approved_at, expires_at, status) values
  ('11111111-1111-1111-1111-111111111111', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved');
insert into safesport_training (member_id, training_type, completed_at, expires_at) values
  ('11111111-1111-1111-1111-111111111111', 'core', '2026-06-01', '2027-06-01');

-- Member #2 (suspended): credentials are current too, but an ACTIVE hold overrides everything.
insert into background_check (member_id, source, approved_at, expires_at, status) values
  ('22222222-2222-2222-2222-222222222222', 'sample-ysa', '2026-01-01', '2028-01-01', 'approved');
insert into safesport_training (member_id, training_type, completed_at, expires_at) values
  ('22222222-2222-2222-2222-222222222222', 'core', '2026-06-01', '2027-06-01');
insert into disciplinary_hold (member_id, source, reason) values
  ('22222222-2222-2222-2222-222222222222', 'safesport', 'synthetic example hold');
