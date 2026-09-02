import "server-only";

export type AuthMode = "demo" | "oidc";

export type WebConfig = {
  deployment: "local" | "public";
  authMode: AuthMode;
  apiBaseUrl: string;
  appBaseUrl: string;
  issuer: string;
  clientId: string;
  clientSecret: string;
  audience: string;
  sessionSecret: string;
  secureCookies: boolean;
};

const PLACEHOLDER_SECRET = "local-oidc-session-secret-change-before-public-deploy";

function required(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}

export function getWebConfig(): WebConfig {
  const hosted = process.env.VERCEL === "1" || Boolean(process.env.RAILWAY_ENVIRONMENT);
  const deployment = process.env.WEB_DEPLOYMENT_ENV ?? (hosted ? "public" : "local");
  if (deployment !== "local" && deployment !== "public") {
    throw new Error("WEB_DEPLOYMENT_ENV must be local or public");
  }
  const authMode = (process.env.WEB_AUTH_MODE ?? "demo") as AuthMode;
  if (authMode !== "demo" && authMode !== "oidc") {
    throw new Error("WEB_AUTH_MODE must be demo or oidc");
  }
  if (deployment === "public" && authMode !== "oidc") {
    throw new Error("public web deployment requires WEB_AUTH_MODE=oidc");
  }

  const apiBaseUrl = process.env.API_BASE_URL?.trim() || "http://localhost:8080";
  const appBaseUrl = process.env.APP_BASE_URL?.trim() || "http://localhost:3000";
  let issuer = "";
  let clientId = "";
  let clientSecret = "";
  let audience = "";
  let sessionSecret = "";
  if (authMode === "oidc") {
    issuer = required("OIDC_ISSUER_URL").replace(/\/$/, "");
    clientId = required("OIDC_CLIENT_ID");
    clientSecret = process.env.OIDC_CLIENT_SECRET?.trim() || "";
    audience = required("OIDC_AUDIENCE");
    sessionSecret = required("SESSION_SECRET");
    if (sessionSecret.length < 32) throw new Error("SESSION_SECRET must contain at least 32 characters");
  }

  if (deployment === "public") {
    for (const [name, value] of [
      ["API_BASE_URL", apiBaseUrl],
      ["APP_BASE_URL", appBaseUrl],
      ["OIDC_ISSUER_URL", issuer],
    ]) {
      if (new URL(value).protocol !== "https:") throw new Error(`${name} must use HTTPS publicly`);
    }
    if (sessionSecret === PLACEHOLDER_SECRET) {
      throw new Error("public deployment cannot use the local SESSION_SECRET placeholder");
    }
    if (!clientSecret) throw new Error("public deployment requires OIDC_CLIENT_SECRET");
  }

  return {
    deployment,
    authMode,
    apiBaseUrl,
    appBaseUrl,
    issuer,
    clientId,
    clientSecret,
    audience,
    sessionSecret,
    secureCookies: deployment === "public" || process.env.SESSION_COOKIE_SECURE === "1",
  };
}

const RETURN_PATHS = new Set(["/", "/learn", "/admin/compliance", "/members"]);

export function safeReturnPath(value: string | null): string {
  return value && RETURN_PATHS.has(value) ? value : "/";
}
