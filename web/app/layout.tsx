import type { Metadata } from "next";
import Link from "next/link";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Learning Center Reference",
  description:
    "A runnable soccer learning, progress, and participation-eligibility reference implementation.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${geistSans.variable} ${geistMono.variable}`}>
      <body>
        <a className="skip-link" href="#main-content">Skip to main content</a>
        <header className="site-header">
          <nav className="nav-shell" aria-label="Primary navigation">
            <Link className="brand" href="/">Learning Center</Link>
            <div className="nav-links">
              <Link href="/learn">Learner</Link>
              <Link href="/admin/compliance">Admin</Link>
              <Link href="/members">Eligibility rules</Link>
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
