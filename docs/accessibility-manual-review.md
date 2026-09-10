# Manual accessibility review

Status: **scaffold — no manual pass has been run yet.** Nothing in this repository claims WCAG
conformance. The automated gate (`web/tests/accessibility.spec.ts`, axe WCAG 2.0/2.1 A and AA
rules on the five rendered routes, run in CI) catches only the subset of failures a machine can
detect: missing names, contrast on flat colours, invalid ARIA, and structure errors. It cannot
judge focus order, whether a screen reader announces a progress update, whether the page still
works at 400% zoom, or whether a focus ring is visible on every control. That is human work.
This document is where that work gets recorded, so a later conformance statement rests on
retained evidence rather than on axe alone.

## How to use this document

1. Start the stack with real sign-in so both signed-out and signed-in states can be reviewed:
   `docker compose -f compose.yml -f compose.oidc.yml up --build`.
2. Work through the **route matrix** below; each row is one state of one route.
3. For every check in the **checklists**, record `pass`, `fail`, or `n/a` in the **findings**
   table, with a one-line note and, for failures, an issue link.
4. Record the environment (browser, screen reader, OS, versions, date, reviewer initials).
5. Only after every row is `pass` or an accepted `n/a` may the README's limitation bullet be
   reworded, and even then the claim should be "reviewed against WCAG 2.2 AA on <date>", not
   "conformant".

Pass criteria reference WCAG 2.2 success criteria (SC). Level A and AA only.

## Route matrix

| # | Route | State to review | How to reach it |
| --- | --- | --- | --- |
| R1 | `/` | signed out | open directly |
| R2 | `/learn` | signed out (sign-in prompt) | open directly |
| R3 | `/learn` | learner, not enrolled | sign in as Alex Coach after `scripts/reset-demo.ps1` |
| R4 | `/learn` | learner, mid-course (progress bar, next lesson, completed lessons) | enroll, mark one lesson complete |
| R5 | `/learn` | learner, course complete | mark all lessons complete |
| R6 | `/admin/compliance` | signed out | open directly |
| R7 | `/admin/compliance` | signed in as learner (not an administrator) | sign in as Alex Coach, then open |
| R8 | `/admin/compliance` | administrator (roster table) | sign out, sign in as Casey Admin |
| R9 | `/members` | signed out | open directly |
| R10 | `/auth/error` | any | open `/api/auth/callback?state=x` to trigger the generic error |
| R11 | OIDC fixture login page | choosing an identity | click any "Sign in" link (local fixture only; not part of the app but part of the journey) |

## Checklists

### K — Keyboard (SC 2.1.1, 2.1.2, 2.4.1, 2.4.3, 2.4.7, 2.4.11, 2.5.8, 3.2.1, 3.2.2)

| ID | Check | SC |
| --- | --- | --- |
| K1 | Every interactive control (links, buttons, the sign-out form, the identity buttons on the fixture) is reachable with Tab/Shift+Tab and operable with Enter/Space. | 2.1.1 |
| K2 | No keyboard trap: focus can always leave every component and return to the browser chrome. | 2.1.2 |
| K3 | The "Skip to main content" link is the first Tab stop, is visible when focused, and moves focus into `<main>`. | 2.4.1 |
| K4 | Focus order follows the visual/reading order: nav → page heading → primary actions → cards in order → footer. | 2.4.3 |
| K5 | The focus indicator is clearly visible on every control against both light and dark surfaces, including the primary buttons, table links, and progress-card buttons. | 2.4.7, 1.4.11 |
| K6 | The focused control is never fully hidden by sticky navigation or an overflow container (roster table at narrow widths). | 2.4.11 |
| K7 | Buttons and links meet the 24×24 CSS px minimum target size or have adequate spacing. | 2.5.8 |
| K8 | Receiving focus never changes context (no auto-navigation on focus); activating "Mark complete" changes only the page's own content. | 3.2.1, 3.2.2 |
| K9 | After "Mark complete" or "Enroll in course", focus lands somewhere sensible (the updated card or the next lesson's button), not on `<body>`. | 2.4.3 |

### S — Screen reader (SC 1.3.1, 1.3.2, 2.4.2, 2.4.4, 2.4.6, 3.1.1, 3.3.1, 3.3.2, 4.1.2, 4.1.3)

Test with at least one of: NVDA + Firefox or Chrome (Windows), VoiceOver + Safari (macOS/iOS),
TalkBack + Chrome (Android). Record which.

| ID | Check | SC |
| --- | --- | --- |
| S1 | Each page has a unique, descriptive `<title>` announced on load. | 2.4.2 |
| S2 | `lang="en"` is on `<html>` and pronunciation is correct. | 3.1.1 |
| S3 | Landmarks: exactly one `main`, a `navigation` with an accessible name, `banner`/`contentinfo` where present; the landmark list is navigable. | 1.3.1 |
| S4 | Heading hierarchy is logical (one `h1`, no skipped levels) on every route/state; card headings are real headings. | 1.3.1, 2.4.6 |
| S5 | Link and button text makes sense out of context ("Open learner workspace", "Mark complete" is followed by the lesson name or the card's name gives context). | 2.4.4 |
| S6 | The progress bar is announced with its name and current value ("Grassroots Match-Day Safety progress, 33 percent") and the value updates after a completion. | 4.1.2 |
| S7 | Lesson completion state (done / next / locked) is conveyed in text, not only by colour or an `aria-hidden` glyph. | 1.3.1, 1.4.1 |
| S8 | Compliance roster: column headers are announced per cell; status and reason are readable; the earliest-expiry cell reads as a date. | 1.3.1 |
| S9 | Members page: each example's status is conveyed in text and the status enum is explained. | 1.3.1 |
| S10 | After "Mark complete" or "Enroll", the change is announced without moving focus away (a live region or an updated focused element), or the resulting page load announces the new heading. | 4.1.3 |
| S11 | The generic sign-in error page explains what happened and offers the recovery action; no provider detail is required to recover. | 3.3.1, 3.3.2 |
| S12 | Decorative arrows and glyphs (`aria-hidden`) are skipped; nothing meaningful is hidden. | 1.3.1 |

### Z — Zoom, reflow, and text spacing (SC 1.4.4, 1.4.10, 1.4.12, 1.4.13)

| ID | Check | SC |
| --- | --- | --- |
| Z1 | 200% browser zoom: all text readable, nothing clipped or overlapping, no lost functionality. | 1.4.4 |
| Z2 | 400% zoom at 1280 px wide (equivalent to 320 CSS px): content reflows to one column with no two-dimensional scrolling, except the roster table, which scrolls horizontally inside its own container. | 1.4.10 |
| Z3 | Text-spacing override (line-height 1.5, paragraph 2×, letter 0.12em, word 0.16em): no clipping or overlap. | 1.4.12 |
| Z4 | Content that appears on hover/focus (if any tooltips are introduced) is dismissible, hoverable, and persistent. | 1.4.13 |
| Z5 | Phone width (~400 px): nav wraps, cards stack, the primary actions remain reachable. | 1.4.10 |

### C — Colour, contrast, and motion (SC 1.4.1, 1.4.3, 1.4.11; 2.3.3 noted as AAA)

Measure with a contrast tool (browser devtools, Colour Contrast Analyser) in both the light and
the dark scheme if the OS setting changes the palette.

| ID | Check | SC |
| --- | --- | --- |
| C1 | Body and secondary text ≥ 4.5:1 against its background; large text ≥ 3:1. | 1.4.3 |
| C2 | Status badges (eligible / suspended / ineligible_lapsed) meet 4.5:1 for their text. | 1.4.3 |
| C3 | Focus indicators, button borders, progress-bar fill vs track, and table borders ≥ 3:1. | 1.4.11 |
| C4 | Status and lesson state are never conveyed by colour alone (text or icon with text present). | 1.4.1 |
| C5 | `prefers-reduced-motion: reduce` removes the progress-bar and hover transitions (project preference; AAA 2.3.3). | 2.3.3 |

## Findings

Fill one row per route-state × check that was actually performed. Leave nothing implied.

| Route | Check | Result | Notes | Issue |
| --- | --- | --- | --- | --- |
| | | | | |

## Environment record

| Field | Value |
| --- | --- |
| Date | |
| Reviewer | |
| OS / version | |
| Browser / version | |
| Screen reader / version | |
| Contrast tool | |
| Commit reviewed | |

## Known limitations before the review starts

- The OIDC fixture login page (R11) is a test dependency, not the product; a hosted provider's
  login page would need its own review and is out of scope.
- The `/members` route renders the raw status enum (e.g. `ineligible_lapsed`) deliberately so the
  end-to-end test can grep it; the manual review should confirm the surrounding text explains it.
- Reduced-motion support and the skip link are implemented; whether they are *sufficient* is what
  this review decides.
- All routes currently share one static `<title>` ("Learning Center Reference", set in
  `web/app/layout.tsx`). Expect S1 to fail until per-route titles are added; record it rather
  than fixing it silently so the review stays an honest log.
