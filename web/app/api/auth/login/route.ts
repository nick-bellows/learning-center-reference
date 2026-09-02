import { NextRequest, NextResponse } from "next/server";
import { getWebConfig, safeReturnPath } from "@/lib/config";
import { pkceChallenge, providerMetadata, randomURLSafe } from "@/lib/oidc";
import { setTransaction } from "@/lib/session";

export async function GET(request: NextRequest) {
  try {
    const config = getWebConfig();
    if (config.authMode !== "oidc") return NextResponse.redirect(new URL("/", request.url));
    const metadata = await providerMetadata();
    const state = randomURLSafe();
    const nonce = randomURLSafe();
    const verifier = randomURLSafe(48);
    const returnTo = safeReturnPath(request.nextUrl.searchParams.get("returnTo"));
    await setTransaction({
      state,
      nonce,
      verifier,
      returnTo,
      expiresAt: Math.floor(Date.now() / 1000) + 600,
    });
    const authorize = new URL(metadata.authorization_endpoint);
    authorize.search = new URLSearchParams({
      response_type: "code",
      client_id: config.clientId,
      redirect_uri: `${config.appBaseUrl}/api/auth/callback`,
      scope: "openid",
      audience: config.audience,
      state,
      nonce,
      code_challenge: pkceChallenge(verifier),
      code_challenge_method: "S256",
      prompt: "login",
    }).toString();
    return NextResponse.redirect(authorize);
  } catch {
    return NextResponse.redirect(new URL("/auth/error?code=login_unavailable", request.url));
  }
}
