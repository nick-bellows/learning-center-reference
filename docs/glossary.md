# Glossary

> Plain-language definitions of terms as they come up. Written so Nick can explain each in an interview. Add to it whenever a new term appears.

### Database / schema
- **Migration** — a versioned file that changes the database schema. `.up.sql` applies the change; `.down.sql` reverses it. Running them in order builds the whole schema from nothing, reproducibly.
- **UUID** — a random 128-bit id (e.g. `a3f1...`). We use it for primary keys instead of `1, 2, 3` so ids aren't guessable/enumerable and don't collide across systems. `gen_random_uuid()` generates one (built into Postgres 13+).
- **`timestamptz`** — "timestamp with time zone." Stores an instant unambiguously. Always prefer it over a plain `timestamp`.
- **Primary key** — the column that uniquely identifies a row.
- **Foreign key** — a column that points at another table's primary key (e.g. `module.course_id` → `course.id`). The database enforces the link.
- **`ON DELETE CASCADE`** — if the parent row is deleted, delete the children too (delete a course → its modules/lessons go with it). **`ON DELETE SET NULL`** — instead null the reference (delete an association → members keep existing, just unlinked).
- **`CHECK` constraint** — a rule the database enforces on a column, e.g. `role in ('learner','instructor',...)`. We use CHECK + text instead of Postgres `ENUM` types because adding a new allowed value later is a one-line change instead of a painful type alter.
- **`UNIQUE` constraint** — forbids duplicate values (e.g. a member enrolls in a given course at most once).
- **Index** — a lookup structure that makes queries on a column fast. **Partial index** — an index over only *some* rows (we index only *active* disciplinary holds, since those are all eligibility ever asks about).
- **Derived / computed value** — a value calculated from other data rather than stored (e.g. "is this member a minor?" from `date_of_birth`; "is this member eligible?" from the safeguarding inputs). Storing derived values invites staleness bugs.

### Domain
- **Prerequisite graph (DAG)** — "directed acyclic graph." The coaching licenses aren't a straight line (A splits into A-Youth / A-Senior), so prerequisites are modeled as a graph, not a chain.
- **Eligibility engine** — the rule that computes participation status from safeguarding inputs + holds. See `docs/domain-model.md` §4.
- **Age group / cohort & cutoff** — a player's competitive age group is derived from `date_of_birth` + the organization's cutoff rule: either **birth-year** based, or **school-year** based with a cutoff date (Aug 1, June 30, etc. — varies by org). A plain "age" can't express this, which is the deeper reason we store DOB, not age.

### Go language
- **Package** — a folder of Go files sharing a namespace (our first is `safeguarding`). Files in the same package see each other's names without imports. Like a C# namespace.
- **Module / `go.mod`** — a Go project's manifest: its import path + Go version + dependencies. `go mod init` creates it. Roughly like a `.csproj` / NuGet manifest.
- **Named type** (`type Status string`) — a distinct type backed by a base type. Lets the compiler stop you mixing a `Status` with a plain string. Combined with `const (...)` it's Go's version of a C# enum, but with readable string values.
- **Struct** — a plain data record with fields (`Inputs`, `Decision`). Like a C# `struct`/POCO. No inheritance in Go — you compose structs instead.
- **Pointer + `nil`** (`*time.Time`) — a pointer can be `nil` ("no value"), so `*time.Time` is Go's nullable date, like C#'s `DateTime?`. `nil` = the item isn't on file.
- **Slice** (`[]string`) — a growable list/array. `len(x)` is its length; `len == 0` means empty.
- **Zero value** — every Go value has a default: `0`, `""`, `nil`, empty slice. An `Inputs{}` with fields omitted uses these — why an omitted `ActiveHoldSources` is simply empty.
- **Table-driven test** — list test cases in a slice, loop over them, run each with `t.Run(name, ...)` as a named subtest. The idiomatic Go testing style; adding a case is one line.
- **`go test ./...`** — compiles and runs every test in the module. Green = all pass.

### Coming later
- **Event log / projection** — (M2) instead of storing "current progress," we store an append-only list of things that happened, then *project* the current state from them.
