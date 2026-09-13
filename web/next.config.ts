import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Standalone output: `next build` emits .next/standalone with a minimal
  // server.js + only the node_modules it needs — what the Dockerfile ships.
  output: "standalone",
  // Strict-Transport-Security is deliberately absent: it belongs on the TLS terminator
  // (the hosting platform or reverse proxy), which knows whether the origin is HTTPS. The
  // local demo is plain HTTP, where browsers ignore the header anyway.
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          // The app loads no third-party script, style, font, or image, so everything but
          // 'self' is denied. 'unsafe-inline' remains for script/style because the App
          // Router emits inline hydration scripts and style attributes; a nonce-based policy
          // (via proxy.ts) is the documented follow-up before any hosted deployment.
          {
            key: "Content-Security-Policy",
            value: [
              "default-src 'self'",
              "script-src 'self' 'unsafe-inline'",
              "style-src 'self' 'unsafe-inline'",
              "img-src 'self' data:",
              "font-src 'self'",
              "connect-src 'self'",
              "object-src 'none'",
              "base-uri 'self'",
              "form-action 'self'",
              "frame-ancestors 'none'",
            ].join("; "),
          },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
          { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
          { key: "X-Permitted-Cross-Domain-Policies", value: "none" },
        ],
      },
    ];
  },
};

export default nextConfig;
