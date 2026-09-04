# Domain Model

> Status: **reference model with a bounded implemented slice.** Learning progress, roles, generic role credentials, safeguarding inputs, and overall eligibility are implemented. Coaching pathways, referee grades, assessment attempts, certificate issuance, and age-cohort rules remain design notes. Modeled from public sources and the author's general domain knowledge; no employer-internal process or data appears here. Real-world requirements change and vary by organization, so this is not an authoritative compliance reference.
>
> **Confidence tags:** `[sourced]` = from a cited public page; `[domain]` = from the author's experience; `[assumption]` = a modeling default to confirm.

---

## 1. Learning Center (content + progress)

**Shape:** `Course` → ordered `Module`s → ordered `Lesson`s; a `Lesson` may have an `Assessment`.

- Lesson types: video, reading, quiz.
- `Assessment` currently stores a passing threshold and attempt limit; attempt history is planned.
- The implemented demo completes a course after all ordered lessons. Requiring a passing
  assessment and issuing a credential/certificate are the next domain increment, not current behavior.

**Rules resolved:**
- **1a. Ordering `[domain]`:** *It depends on the course, but typically in order.* → model an `ordering` flag on a course: `sequential` (must complete lessons in order — the default) vs `open` (any order). This one flag captures both cases cleanly.
- **1d. Where expiry lives:** on the **credential** (license/certification), not on the assessment score. A passed assessment is a historical fact; what expires is the license it fed into (§2).

---

## 2. Coaching Education (licenses)

**Pathway `[sourced]`** ([US Club Soccer — U.S. Soccer Education](https://usclubsoccer.org/ussoccer-coach-education), [Cal South pathway](https://calsouth.com/coaching-license-pathway/)):

```
Grassroots (online modules + in-person modules: 4v4 → 7v7 → 9v9 → 11v11)
   → D License → C License → B License → A License (A-Youth | A-Senior) → Pro License
```

- **Grassroots:** entry level; minimum age 16; the four in-person game models + online courses.
- **D:** entry requires the Grassroots in-person **11v11** (or an E license) + one other in-person Grassroots module (4v4/7v7/9v9) + a Grassroots online course.
- **C:** requires a **D license held ≥ 12 months**.
- **B:** youth (U13+) / senior performance; application-based entry.
- **A:** splits into **A-Youth** and **A-Senior**.
- **Pro:** requires **A-Senior** + significant professional experience.

**Modeling decision:** represent prerequisites as a **directed graph** (each level lists prerequisite level(s) + conditions like "held ≥ 12 months"), not a hardcoded chain — because A splits into two tracks and Grassroots has module combinations. This is the "prerequisite chain vs partial order" question answered: **partial order (DAG).**

**Expiry & renewal `[sourced]`** ([US Soccer CPD FAQ](https://s3.us-east-2.amazonaws.com/aws-s2-images.ussoccer.com/coaches/CPD-FAQs-Coaches.pdf)):
- **Grassroots licenses do NOT expire.**
- **C, B, A, Pro** require **Continuing Education Units (CEUs) within a rolling 3-year cycle**; miss the CEUs → license marked **expired**. Excess CEUs do **not** roll into the next cycle.
- **Modeling decision:** a `License` carries `issued_at`, an optional `expires_at` (null for Grassroots), and (for C+) a `renewal_window` with a required CEU count. CEUs are records tied to the license's current cycle.

---

## 3. Refereeing (recertification)

**Grades `[sourced]`** ([Metro DC-VA 2026 requirements](https://vadcsoccerref.demosphere-secure.com/instruction/2026-referee-certification-requirements-summary)): **Referee → Regional → National → Professional**.

- **Annual recertification** for all grades; advanced grades add fitness/testing/in-service requirements.
- Recert components (typical): online module, recertifying quiz, Laws of the Game update.
- **Background screening every 2 years** for referees 18+ (same as §4).
- Registration/seasonal year runs ~**July 1 – June 30**.

**Modeling decision:** a referee holds a `Grade` with an annual `recertification` record keyed to the seasonal year (has a `window_opens` / `window_closes` and a `completed_at`). Status between cycles = current; after the window closes without completion = lapsed.

---

## 4. Safeguarding — the derived-eligibility engine (the moat)

This is the differentiator. Whether a member may **participate** is **not stored** — it is **computed** from several independently-expiring inputs plus any disciplinary hold.

**Inputs `[domain]` + `[sourced]`:**
1. **SafeSport certification** `[sourced]` — required for adults (and players turning 18 in the seasonal year). SafeSport **Trained Core** before contact with minors / within 45 days of membership, then **annual refreshers** (Core again in year 5). Under USSF Policy 212-3. ([US Soccer training FAQ](https://www.ussoccer.com/safeguarding/training-faq)) → treat as an **annually-expiring** credential.
2. **Background check** `[sourced]` — required for adults; **valid 2 years**, renewed every 2 years. ([STX Referees](https://www.stxref.org/news/annual-safesport-training-background-check/)) Issued by the local club/association/organization `[domain]`.
3. **Disciplinary hold / flag** `[domain]` — if a crime or inappropriate act is alleged or committed, an individual may be **flagged/suspended** by **U.S. Soccer, the U.S. Center for SafeSport, or a local organization**, depending on where the report originates. An active hold makes them **ineligible regardless of everything else.**
4. **Role credential** — coach: a current required license (§2); referee: current recertification (§3).

**The eligibility rule (this becomes the core function + its tests):**

> An **adult in an adult role** is **eligible to participate** iff **all** hold:
> - **No active disciplinary hold** (overrides everything), **AND**
> - **SafeSport certification current** (not lapsed), **AND**
> - **Background check current** (within 2-year validity), **AND**
> - the **role credential** is current (license for coaches / recertification for referees).
>
> If **any** input lapses, or a hold is active, status flips to **ineligible automatically** — no separate action sets it. It's a computed value over the inputs' dates and holds.

**Implemented status values:**
- `eligible` — all inputs current, no hold.
- `ineligible_lapsed` — a required credential has expired.
- `suspended` — an active disciplinary hold.

`provisional` is deliberately not implemented because the current schema does not contain
enough workflow state to distinguish pending review from missing evidence.

**Grace period:** default **none**. The pure Go rule accepts an explicit `grace_days` input and
tests the boundary, but the store currently supplies zero because no organization policy is modeled.

**Player exclusion `[domain]`:** players who are minors (18U/19U, etc.) are **not** subject to the adult SafeSport/background-check requirements. Eligibility rules above apply to **adult roles**. Minor-player rules are out of scope beyond noting the exclusion.

**Age & cohort determination `[domain]`:** whether someone is a *minor* — and, for players, which *age group* they fall in — is derived from **`date_of_birth` + the organization's cutoff rule**, never from a stored `age`. Cohorts are defined either by **birth year** or by **school year with an org-specific cutoff date** (e.g. Aug 1, June 30 — it varies by association/competition). A plain age can't express this distinction. It also sharpens the SafeSport rule ("players turning 18 *during the seasonal year*"): the minor/adult line keys off the **seasonal year + cutoff**, not today's date. Model the cutoff as a rule on the association/competition and derive minor status + age group from it. *(Schema already stores `date_of_birth`; the cutoff rule is added when age-group logic is needed.)*

**Administrator expiration view:** the implemented dashboard exposes each member's earliest
credential expiration. Separate 30/60/90-day groupings and notification jobs are planned.

---

## 5. Roles & actors

- **Learner** — a coach or referee taking courses / earning credentials.
- **Instructor** — teaches/grades courses.
- **Admin** — sees compliance across members; manages holds/records.
- **Member association** `[sourced]` — courses are searched/registered **by association, level, or location** ([US Soccer LC launch](https://www.ussoccer.com/stories/2019/06/us-soccer-learning-center-launch-marks-new-era-of-member-service)); model it as a light first-class entity that scopes courses and members.
- **Multi-role:** one person can hold several roles. The implemented overall participation
  decision uses the weakest required role credential; a per-role decision endpoint is future work.

---

## 6. Implemented boundary

1. **Implemented:** member/role resolution, course/module/lesson/assessment metadata,
   enrollment, ordered completion events, progress projection, generic role credentials,
   safeguarding records, and derived overall eligibility.
2. **Not implemented:** assessment attempts/passing, certificate or credential issuance from
   course completion, coaching prerequisite DAG/CEUs, referee grades, and cohort logic.

---

## 7. Decisions recorded for this slice

- [x] Use only eligible / lapsed / suspended until a real pending-review workflow exists.
- [x] Use no implicit grace period; any future policy must supply it explicitly.
- [x] Start with a generic referee credential; defer grades.
- [x] Retain a constrained disciplinary-hold source field.
