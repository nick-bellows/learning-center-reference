"use client";

// Root-level error boundary. `app/error.tsx` cannot catch a failure in the root layout
// itself (for example, an invalid deployment configuration surfacing from getWebConfig at
// request time), so this file replaces the whole document in that case. It must render its
// own <html> and <body> and cannot rely on globals.css, hence the inline styles. No error
// detail is shown; the server log carries it.
export default function GlobalError({ retry }: { error: Error & { digest?: string }; retry: () => void }) {
  return (
    <html lang="en">
      <head>
        <title>Service error | Learning Center Reference</title>
      </head>
      <body
        style={{
          margin: 0,
          fontFamily: "system-ui, sans-serif",
          background: "#f4f7fb",
          color: "#0f172a",
        }}
      >
        <main style={{ maxWidth: "40rem", margin: "0 auto", padding: "4rem 1.5rem" }}>
          <p style={{ color: "#2457a7", fontWeight: 700, textTransform: "uppercase", fontSize: "0.8rem" }}>
            Something went wrong
          </p>
          <h1 style={{ fontSize: "2rem", lineHeight: 1.2 }}>The Learning Center is unavailable</h1>
          <p style={{ fontSize: "1.1rem", lineHeight: 1.6 }}>
            The page could not be rendered. If this persists, the service configuration needs
            attention; no further detail is shown here.
          </p>
          <button
            type="button"
            onClick={() => retry()}
            style={{
              marginTop: "1.5rem",
              padding: "0.75rem 1.25rem",
              borderRadius: "0.5rem",
              border: "none",
              background: "#0c2340",
              color: "#ffffff",
              fontSize: "1rem",
              cursor: "pointer",
            }}
          >
            Try again
          </button>
        </main>
      </body>
    </html>
  );
}
