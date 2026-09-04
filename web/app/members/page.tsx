import { getWebConfig } from "@/lib/config";

// The three fixed synthetic examples for the public, unauthenticated eligibility endpoint.
// The authenticated administrator roster comes from /v1/admin/compliance instead.
const DEMO_MEMBERS = [
  { id: "11111111-1111-1111-1111-111111111111", label: "Alex Coach" },
  { id: "22222222-2222-2222-2222-222222222222", label: "Sam Referee" },
  { id: "33333333-3333-3333-3333-333333333333", label: "Riley Referee" },
];

type Eligibility = {
  member_id: string;
  status: "eligible" | "suspended" | "ineligible_lapsed";
  reason: string;
};

async function fetchEligibility(id: string): Promise<Eligibility | null> {
  try {
    const res = await fetch(`${getWebConfig().apiBaseUrl}/v1/members/${id}/eligibility`, {
      cache: "no-store",
    });
    if (!res.ok) return null;
    return (await res.json()) as Eligibility;
  } catch {
    return null;
  }
}

function statusClass(status: string | undefined): string {
  if (status === "eligible") return "status status-good";
  if (status === "suspended") return "status status-danger";
  if (status === "ineligible_lapsed") return "status status-warning";
  return "status status-active";
}

export default async function MembersPage() {
  const rows = await Promise.all(
    DEMO_MEMBERS.map(async (member) => ({ ...member, eligibility: await fetchEligibility(member.id) })),
  );

  return (
    <main className="page-shell">
      <p className="eyebrow">Eligibility rules</p>
      <h1>Member participation status</h1>
      <p className="lede max-w-2xl">
        Eligibility is computed live from each member&rsquo;s safeguarding records.
      </p>

      <section className="card mt-8 overflow-x-auto" aria-labelledby="examples-title">
        <div className="mb-5">
          <p className="eyebrow">Live rules evaluation</p>
          <h2 id="examples-title">Fixed synthetic examples</h2>
        </div>
        <table>
          <thead>
            <tr>
              <th>Member</th>
              <th>Status</th>
              <th>Reason</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id}>
                <td className="font-medium text-slate-950">{row.label}</td>
                <td>
                  {/* This troubleshooting view shows the literal rule output (e.g.
                      ineligible_lapsed), not a humanized label. */}
                  <span className={statusClass(row.eligibility?.status)}>
                    {row.eligibility ? row.eligibility.status : "unknown"}
                  </span>
                </td>
                <td className="text-slate-600">{row.eligibility?.reason ?? "API unavailable"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </main>
  );
}
