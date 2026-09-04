import Link from "next/link";

const repository = "https://github.com/nick-bellows/learning-center-reference";

export default function Home() {
  return (
    <main className="page-shell space-y-12">
      <section className="home-hero">
        <div>
          <p className="eyebrow">Learning Center Reference</p>
          <h1>Learning progress and participation eligibility, connected.</h1>
          <p className="lede max-w-3xl">
            A small, runnable reference implementation for the multi-role education and
            compliance workflows a soccer organization has to operate reliably.
          </p>
          <div className="mt-7 flex flex-wrap gap-3">
            <Link className="button button-primary" href="/learn">Open learner workspace</Link>
            <Link className="button button-secondary" href="/admin/compliance">Open admin compliance</Link>
          </div>
        </div>
        <div className="workflow-card" aria-label="Implemented workflow">
          <span>Verified identity</span><b aria-hidden="true">&rarr;</b>
          <span>Database role</span><b aria-hidden="true">&rarr;</b>
          <span>Course enrollment</span><b aria-hidden="true">&rarr;</b>
          <span>Progress events</span><b aria-hidden="true">&rarr;</b>
          <span>Eligibility view</span>
        </div>
      </section>

      <section aria-labelledby="evidence-title">
        <p className="eyebrow">Implemented evidence</p>
        <h2 id="evidence-title">One coherent vertical slice</h2>
        <div className="mt-5 grid gap-4 md:grid-cols-3">
          <article className="card">
            <h3>Authenticated workflow</h3>
            <p>OIDC verification in production mode; explicit synthetic identities locally. Roles resolve from PostgreSQL.</p>
          </article>
          <article className="card">
            <h3>Traceable progress</h3>
            <p>Retry-safe completion events update a transactional learner-dashboard projection without duplicating credit.</p>
          </article>
          <article className="card">
            <h3>Derived compliance</h3>
            <p>Safeguarding, background checks, role credentials, and holds determine eligibility at read time.</p>
          </article>
        </div>
      </section>

      <section aria-labelledby="tour-title">
        <p className="eyebrow">Three-minute code tour</p>
        <h2 id="tour-title">Follow the behavior into the implementation</h2>
        <p className="lede max-w-3xl">
          Each stop below points to working behavior, the code that enforces it, and the test
          that protects the boundary.
        </p>
        <ol className="tour-grid mt-5">
          <li className="card">
            <span className="tour-step">01</span>
            <h3>Sign in and resolve a role</h3>
            <p>Use the learner path, then inspect OIDC verification and database-owned roles.</p>
            <div className="evidence-links">
              <Link href="/learn">Open learner UI</Link>
              <a href={`${repository}/blob/main/api/internal/authn/authn.go`}>Read verifier</a>
              <a href={`${repository}/blob/main/api/internal/httpapi/router.go`}>Trace authorization</a>
            </div>
          </li>
          <li className="card">
            <span className="tour-step">02</span>
            <h3>Enroll and record progress</h3>
            <p>Retry-safe writes append a bounded event and update the projection atomically.</p>
            <div className="evidence-links">
              <a href={`${repository}/blob/main/api/internal/store/store.go`}>Read transaction</a>
              <a href={`${repository}/blob/main/api/migrations/0005_progress.up.sql`}>Read schema</a>
              <a href={`${repository}/blob/main/api/internal/store/store_test.go`}>Read integration test</a>
            </div>
          </li>
          <li className="card">
            <span className="tour-step">03</span>
            <h3>Inspect derived eligibility</h3>
            <p>Compare the administrator roster with the rule and its date-boundary tests.</p>
            <div className="evidence-links">
              <Link href="/admin/compliance">Open admin UI</Link>
              <a href={`${repository}/blob/main/api/internal/safeguarding/eligibility.go`}>Read rule</a>
              <a href={`${repository}/blob/main/api/internal/safeguarding/eligibility_test.go`}>Read tests</a>
            </div>
          </li>
        </ol>
      </section>

      <section className="stack-strip" aria-label="Technology stack">
        <span>Go</span><span>Next.js</span><span>TypeScript</span><span>PostgreSQL</span>
        <span>OpenAPI</span><span>Docker</span><span>GitHub Actions</span>
      </section>
    </main>
  );
}
