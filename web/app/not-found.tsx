import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "Page not found" };

// Rendered inside the root layout for any unmatched route, so a mistyped URL keeps the
// site header, footer, and styles instead of Next's unstyled default.
export default function NotFound() {
  return (
    <main className="page-shell">
      <p className="eyebrow">Not found</p>
      <h1>There is no page at this address</h1>
      <p className="lede max-w-2xl">
        The reference implementation has four pages: the overview, the learner dashboard, the
        administrator compliance view, and the eligibility rule examples.
      </p>
      <Link className="button button-primary mt-6" href="/">
        Back to the overview
      </Link>
    </main>
  );
}
