import { NextRequest, NextResponse } from "next/server";
import { getWebConfig } from "@/lib/config";
import { exchangeCode, providerMetadata, verifyIDToken } from "@/lib/oidc";
import { setSession, takeTransaction, valuesMatch } from "@/lib/session";

export async function GET(request: NextRequest) {
  const failure = () => {
    let base = request.url;
    try {
      base = getWebConfig().appBaseUrl;
    } catch {
      // Configuration failures still return a generic local error page.
    }
    return NextResponse.redirect(new URL("/auth/error?code=callback_rejected", base));
  };
  try {
    const config = getWebConfig();
    if (config.authMode !== "oidc") return failure();
    const code = request.nextUrl.searchParams.get("code") ?? "";
    const state = request.nextUrl.searchParams.get("state") ?? "";
    const transaction = await takeTransaction();
    if (!code || !transaction || !valuesMatch(state, transaction.state)) return failure();

    const metadata = await providerMetadata();
    const tokens = await exchangeCode(metadata, code, transaction.verifier);
    const subject = await verifyIDToken(metadata, tokens.id_token, transaction.nonce);
    const lifetime = Math.min(Math.max(tokens.expires_in, 1), 3_600);
    await setSession({
      accessToken: tokens.access_token,
      subject,
      expiresAt: Math.floor(Date.now() / 1000) + lifetime,
    });
    return NextResponse.redirect(new URL(transaction.returnTo, config.appBaseUrl));
  } catch {
    return failure();
  }
}
