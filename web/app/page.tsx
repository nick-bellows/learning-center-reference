import Link from "next/link";

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
          <span>Verified identity</span><b aria-hidden="true">→</b>
          <span>Database role</span><b aria-hidden="true">→</b>
          <span>Course enrollment</span><b aria-hidden="true">→</b>
          <span>Progress events</span><b aria-hidden="true">→</b>
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

      <section className="stack-strip" aria-label="Technology stack">
        <span>Go</span><span>Next.js</span><span>TypeScript</span><span>PostgreSQL</span>
        <span>OpenAPI</span><span>Docker</span><span>GitHub Actions</span>
      </section>
    </main>
  );
}
