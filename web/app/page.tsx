import Link from "next/link";

export default function Home() {
  return (
    <main className="mx-auto max-w-3xl p-8">
      <h1 className="text-3xl font-semibold">Learning Center Reference</h1>
      <p className="mt-2 text-gray-600">
        A portfolio reference implementation of a soccer-federation learning center:
        course delivery, coaching-education licensing, referee recertification, and
        safeguarding-based participation eligibility.
      </p>

      <Link
        href="/members"
        className="mt-6 inline-block rounded-md bg-black px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
      >
        View member eligibility &rarr;
      </Link>

      <p className="mt-12 text-xs text-gray-400">
        Independent portfolio project &mdash; not affiliated with, endorsed by, or
        containing data from U.S. Soccer or any member organization. All data is fictional.
      </p>
    </main>
  );
}
