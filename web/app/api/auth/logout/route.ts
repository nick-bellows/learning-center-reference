import { NextRequest, NextResponse } from "next/server";
import { getWebConfig } from "@/lib/config";
import { clearSession } from "@/lib/session";

// Server Actions get an Origin check from Next for free; this hand-rolled route handler has
// to do its own. SameSite=Lax keeps the cookie off a cross-site POST, but the response's
// Set-Cookie clearing the session is still honoured on a top-level navigation, so without this
// check any page could force a sign-out by auto-submitting a form here.
function isSameOrigin(request: NextRequest, appBaseUrl: string): boolean {
  const origin = request.headers.get("origin");
  if (origin) return origin === new URL(appBaseUrl).origin;
  const site = request.headers.get("sec-fetch-site");
  return site === "same-origin" || site === "none";
}

export async function POST(request: NextRequest) {
  const config = getWebConfig();
  if (!isSameOrigin(request, config.appBaseUrl)) {
    return NextResponse.json({ error: "cross-origin logout rejected" }, { status: 403 });
  }
  await clearSession();
  return NextResponse.redirect(new URL("/", config.appBaseUrl));
}
