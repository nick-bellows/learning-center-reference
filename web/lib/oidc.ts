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
  token_type?: string;
  expires_in?: number;
};

// DEFAULT_TOKEN_LIFETIME_SECONDS is used when the provider omits expires_in (RECOMMENDED,
// not REQUIRED, by RFC 6749). The API independently enforces the token's own exp claim.
export const DEFAULT_TOKEN_LIFETIME_SECONDS = 900;

export function randomURLSafe(bytes = 32): string {
  return randomBytes(bytes).toString("base64url");
}

export function pkceChallenge(verifier: string): string {
  return createHash("sha256").update(verifier).digest("base64url");
}

// Discovery and the JWKS are per issuer, not per request: re-fetching them on every login
// and callback cost two provider round-trips per sign-in and discarded jose's key cache.
// The metadata is re-read after DISCOVERY_TTL_MS; the JWKS set refreshes itself on an
// unknown key id.
const DISCOVERY_TTL_MS = 10 * 60 * 1000;
type ProviderCache = {
  issuer: string;
  metadata: ProviderMetadata;
  jwks: ReturnType<typeof createRemoteJWKSet>;
  fetchedAt: number;
};
let providerCache: ProviderCache | null = null;

export async function providerMetadata(): Promise<ProviderMetadata> {
  const config = getWebConfig();
  const cached = providerCache;
  if (cached && cached.issuer === config.issuer && Date.now() - cached.fetchedAt < DISCOVERY_TTL_MS) {
    return cached.metadata;
  }
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
  providerCache = {
    issuer: config.issuer,
    metadata,
    jwks: createRemoteJWKSet(new URL(metadata.jwks_uri)),
    fetchedAt: Date.now(),
  };
  return metadata;
}

function jwksFor(metadata: ProviderMetadata) {
  if (providerCache && providerCache.metadata.jwks_uri === metadata.jwks_uri) return providerCache.jwks;
  return createRemoteJWKSet(new URL(metadata.jwks_uri));
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
  const tokenType = tokens.token_type ?? "bearer";
  if (!tokens.access_token || !tokens.id_token || tokenType.toLowerCase() !== "bearer") {
    throw new Error("identity provider returned incomplete tokens");
  }
  return tokens;
}

// tokenLifetimeSeconds bounds the session lifetime to a sane window even when the provider
// omits or reports a nonsensical expires_in, so the session's own expiry can never be NaN.
export function tokenLifetimeSeconds(expiresIn: number | undefined): number {
  const seconds = Number(expiresIn);
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return DEFAULT_TOKEN_LIFETIME_SECONDS;
  }
  return Math.min(Math.floor(seconds), 3_600);
}

export async function verifyIDToken(
  metadata: ProviderMetadata,
  rawToken: string,
  nonce: string,
): Promise<string> {
  const config = getWebConfig();
  // Only asymmetric signatures are acceptable for a token verified against a public JWKS.
  const result = await jwtVerify(rawToken, jwksFor(metadata), {
    issuer: config.issuer,
    audience: config.clientId,
    algorithms: ["RS256", "ES256", "PS256"],
  });
  if (!result.payload.sub || result.payload.nonce !== nonce) {
    throw new Error("identity token subject or nonce is invalid");
  }
  return result.payload.sub;
}
