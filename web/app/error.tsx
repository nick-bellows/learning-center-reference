"use client";

// Route-level error boundary. A Server Action that hits an unexpected API error (for
// example, submitting a stale lesson form that the API rejects with 409) lands here with a
// recoverable "Try again" instead of an unhandled crash. No error detail is shown to the user.
export default function Error({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main className="page-shell">
      <p className="eyebrow">Something went wrong</p>
      <h1>This action could not be completed</h1>
      <p className="lede max-w-2xl">
        The page may have been out of date. Reload the latest state and try again.
      </p>
      <button className="button button-primary mt-6" type="button" onClick={() => reset()}>
        Try again
      </button>
    </main>
  );
}
