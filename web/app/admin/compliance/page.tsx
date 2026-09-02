import { getCompliance } from "@/lib/api";

const dateFormatter = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
  year: "numeric",
  timeZone: "UTC",
});

function statusClass(status: string): string {
  if (status === "eligible") return "status status-good";
  if (status === "suspended") return "status status-danger";
  return "status status-warning";
}

export default async function CompliancePage() {
  let members;
  try {
    members = await getCompliance();
  } catch {
    return (
      <main className="page-shell">
        <p className="eyebrow">Administrator workspace</p>
        <h1>Compliance service unavailable</h1>
        <p className="lede">Start the local API and PostgreSQL services, then reload this page.</p>
      </main>
    );
  }

  const eligible = members.filter((member) => member.status === "eligible").length;
  const review = members.length - eligible;

  return (
    <main className="page-shell space-y-8">
      <section className="hero-panel">
        <div>
          <p className="eyebrow">Administrator workspace</p>
          <h1>Participation compliance</h1>
          <p className="lede max-w-2xl">
            Status is calculated from credential expirations and active holds each time this
            page loads. There is no editable “eligible” flag to become stale.
          </p>
        </div>
        <span className="role-chip">admin</span>
      </section>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="metric-card"><strong>{eligible}</strong><span>currently eligible</span></div>
        <div className="metric-card"><strong>{review}</strong><span>need attention</span></div>
      </div>

      <section className="card overflow-x-auto" aria-labelledby="roster-title">
        <div className="mb-5">
          <p className="eyebrow">Live rules evaluation</p>
          <h2 id="roster-title">Synthetic member roster</h2>
        </div>
        <table>
          <thead>
            <tr>
              <th>Member</th>
              <th>Roles</th>
              <th>Status</th>
              <th>Earliest expiry</th>
              <th>Reason</th>
            </tr>
          </thead>
          <tbody>
            {members.map((member) => (
              <tr key={member.id}>
                <td className="font-medium text-slate-950">{member.display_name}</td>
                <td>{member.roles.join(", ") || "none"}</td>
                <td><span className={statusClass(member.status)}>{member.status.replace("_", " ")}</span></td>
                <td>{member.next_expiration ? dateFormatter.format(new Date(member.next_expiration)) : "Not on file"}</td>
                <td className="max-w-sm">{member.reason}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </main>
  );
}
