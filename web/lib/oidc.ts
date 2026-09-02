import "server-only";

import { createHash, randomBytes } from "node:crypto";
import { createRemoteJWKSet, jwtVerify } from "jose";
import { getWebConfig } from "./config";

export type ProviderMetadata = {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  jwks_uri: string;
};

export type TokenResponse = {
  access_token: string;
  id_token: string;
  token_type: string;
  expires_in: number;
};

export function randomURLSafe(bytes = 32): string {
  return randomBytes(bytes).toString("base64url");
}

export function pkceChallenge(verifier: string): string {
  return createHash("sha256").update(verifier).digest("base64url");
}

export async function providerMetadata(): Promise<ProviderMetadata> {
  const config = getWebConfig();
  const response = await fetch(`${config.issuer}/.well-known/openid-configuration`, {
    cache: "no-store",
    signal: AbortSignal.timeout(5_000),
  });
  if (!response.ok) throw new Error("identity provider discovery failed");
  const metadata = (await response.json()) as ProviderMetadata;
  if (metadata.issuer !== config.issuer) throw new Error("identity provider issuer mismatch");
  for (const endpoint of [
    metadata.authorization_endpoint,
    metadata.token_endpoint,
    metadata.jwks_uri,
  ]) {
    if (!endpoint) throw new Error("identity provider metadata is incomplete");
  }
  return metadata;
}

export async function exchangeCode(
  metadata: ProviderMetadata,
  code: string,
  verifier: string,
): Promise<TokenResponse> {
  const config = getWebConfig();
  const response = await fetch(metadata.token_endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "authorization_code",
      client_id: config.clientId,
      ...(config.clientSecret ? { client_secret: config.clientSecret } : {}),
      code,
      code_verifier: verifier,
      redirect_uri: `${config.appBaseUrl}/api/auth/callback`,
    }),
    cache: "no-store",
    signal: AbortSignal.timeout(5_000),
  });
  if (!response.ok) throw new Error("authorization code exchange failed");
  const tokens = (await response.json()) as TokenResponse;
  if (!tokens.access_token || !tokens.id_token || tokens.token_type.toLowerCase() !== "bearer") {
    throw new Error("identity provider returned incomplete tokens");
  }
  return tokens;
}

export async function verifyIDToken(
  metadata: ProviderMetadata,
  rawToken: string,
  nonce: string,
): Promise<string> {
  const config = getWebConfig();
  const result = await jwtVerify(rawToken, createRemoteJWKSet(new URL(metadata.jwks_uri)), {
    issuer: config.issuer,
    audience: config.clientId,
  });
  if (!result.payload.sub || result.payload.nonce !== nonce) {
    throw new Error("identity token subject or nonce is invalid");
  }
  return result.payload.sub;
}
