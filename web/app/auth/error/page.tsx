export default function AuthenticationErrorPage() {
  return (
    <main className="page-shell">
      <p className="eyebrow">Authentication</p>
      <h1>Sign-in could not be completed</h1>
      <p className="lede max-w-2xl">
        The login transaction was missing, expired, or rejected. No provider detail or token is
        shown here. Start a fresh sign-in to try again.
      </p>
      <a className="button button-primary mt-6" href="/api/auth/login">
        Start a fresh sign-in
      </a>
    </main>
  );
}
