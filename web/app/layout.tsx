import type { Metadata } from "next";
import Link from "next/link";
import { getViewerState } from "@/lib/api";
import "./globals.css";

export const metadata: Metadata = {
  title: "Learning Center Reference",
  description:
    "A runnable soccer learning, progress, and participation-eligibility reference implementation.",
};

export default async function RootLayout({ children }: LayoutProps<"/">) {
  const viewer = await getViewerState();
  return (
    <html lang="en">
      <body>
        <a className="skip-link" href="#main-content">Skip to main content</a>
        <header className="site-header">
          <nav className="nav-shell" aria-label="Primary navigation">
            <Link className="brand" href="/">Learning Center</Link>
            <div className="nav-links">
              <Link href="/learn">Learner</Link>
              <Link href="/admin/compliance">Admin</Link>
              <Link href="/members">Eligibility rules</Link>
              {viewer.authMode === "oidc" && viewer.signedIn ? (
                <form action="/api/auth/logout" method="post">
                  <button className="nav-auth" type="submit">Sign out</button>
                </form>
              ) : viewer.authMode === "oidc" ? (
                <a className="nav-auth" href="/api/auth/login">Sign in</a>
              ) : (
                <span className="nav-mode">local demo</span>
              )}
            </div>
          </nav>
        </header>
        <div id="main-content">{children}</div>
        <footer className="site-footer">
          Independent portfolio reference implementation. Fictional data only. Not affiliated
          with or endorsed by U.S. Soccer or any member organization.
        </footer>
      </body>
    </html>
  );
}
