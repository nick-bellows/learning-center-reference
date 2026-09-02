// A Server Component: it runs on the server, so it can call our API directly and never
// ships this code (or the API URL) to the browser. In Next 16 a component is a Server
// Component by default; making it `async` lets us `await` data right here.
//
// This rules-focused page intentionally names the three fixed synthetic examples.
// The authenticated administrator roster comes from /v1/admin/compliance instead.

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080";

const DEMO_MEMBERS = [
  { id: "11111111-1111-1111-1111-111111111111", label: "Alex Coach" },
  { id: "22222222-2222-2222-2222-222222222222", label: "Sam Referee" },
  { id: "33333333-3333-3333-3333-333333333333", label: "Riley Referee" },
];

// A TypeScript type describing the API's JSON shape. `status` is a union of the exact
// strings the API can return — like a C# enum, checked at compile time.
type Eligibility = {
  member_id: string;
  status: "eligible" | "suspended" | "ineligible_lapsed";
  reason: string;
};

async function fetchEligibility(id: string): Promise<Eligibility | null> {
  try {
    // cache: "no-store" = always ask the API fresh (this is the default in Next 16, but
    // we're explicit). If the API is down we return null instead of crashing the page.
    const res = await fetch(`${API_BASE}/v1/members/${id}/eligibility`, {
      cache: "no-store",
    });
    if (!res.ok) return null;
    return (await res.json()) as Eligibility;
  } catch {
    return null;
  }
}

// A small presentational component. Props are typed inline. It just maps a status to
// Tailwind color classes and renders a pill.
function StatusBadge({ status }: { status?: string }) {
  const styles: Record<string, string> = {
    eligible: "bg-green-100 text-green-800 ring-green-600/20",
    suspended: "bg-red-100 text-red-800 ring-red-600/20",
    ineligible_lapsed: "bg-amber-100 text-amber-800 ring-amber-600/20",
  };
  const cls = (status && styles[status]) || "bg-gray-100 text-gray-700 ring-gray-500/20";
  return (
    <span className={`inline-flex rounded-full px-2 py-1 text-xs font-medium ring-1 ring-inset ${cls}`}>
      {status ?? "unknown"}
    </span>
  );
}

export default async function MembersPage() {
  // Fetch both members in parallel, then wait for all to finish.
  const rows = await Promise.all(
    DEMO_MEMBERS.map(async (m) => ({ ...m, elig: await fetchEligibility(m.id) })),
  );

  return (
    <main className="page-shell">
      <p className="eyebrow">Eligibility rules</p>
      <h1>Member participation status</h1>
      <p className="lede max-w-2xl">
        Eligibility is computed live from each member&rsquo;s safeguarding records.
      </p>

      <div className="card mt-8 overflow-x-auto">
      <table>
        <thead>
          <tr className="border-b border-gray-200">
            <th className="py-2 pr-4 font-medium">Member</th>
            <th className="py-2 pr-4 font-medium">Status</th>
            <th className="py-2 font-medium">Reason</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id} className="border-b border-gray-100">
              <td className="py-3 pr-4">{r.label}</td>
              <td className="py-3 pr-4">
                <StatusBadge status={r.elig?.status} />
              </td>
              <td className="py-3 text-gray-600">{r.elig?.reason ?? "API unavailable"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      </div>
    </main>
  );
}
