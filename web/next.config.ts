import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Standalone output: `next build` emits .next/standalone with a minimal
  // server.js + only the node_modules it needs — what the Dockerfile ships.
  output: "standalone",
};

export default nextConfig;
