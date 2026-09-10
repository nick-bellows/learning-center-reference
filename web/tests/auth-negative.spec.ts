import { createCipheriv, createHash, randomBytes } from "node:crypto";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

// Negative paths for the custom OIDC session/callback code. auth.spec.ts proves the happy
// path; these prove that every way the browser boundary can be abused or go stale ends in a
// signed-out state or the generic error page, never a session and never a leak of provider
// detail. They need the local OIDC Compose overlay, like auth.spec.ts.
test.skip(process.env.PLAYWRIGHT_OIDC !== "1", "requires the local OIDC Compose overlay");

const SESSION_COOKIE = "lcr_session";
const TRANSACTION_COOKIE = "lcr_oidc_transaction";

// The overlay's fixed local secret (compose.oidc.yml). Tests that must forge a validly sealed
// cookie use it; a public deployment refuses to start with this value, so nothing here works
// against a real deployment.
const SESSION_SECRET =
  process.env.SESSION_SECRET ?? "local-oidc-session-secret-change-before-public-deploy";

// Mirrors web/lib/session.ts: AES-256-GCM under sha256(secret), iv.tag.ciphertext (base64url).
function seal(value: object, secret = SESSION_SECRET): string {
  const key = createHash("sha256").update(secret, "utf8").digest();
  const iv = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", key, iv);
  const ciphertext = Buffer.concat([cipher.update(JSON.stringify(value), "utf8"), cipher.final()]);
  return [iv, cipher.getAuthTag(), ciphertext].map((part) => part.toString("base64url")).join(".");
}

async function setCookie(context: BrowserContext, name: string, value: string) {
  await context.addCookies([{ name, value, url: baseURL(), httpOnly: true, sameSite: "Lax" }]);
}

function baseURL(): string {
  return process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:3000";
}

async function cookie(context: BrowserContext, name: string) {
  return (await context.cookies()).find((c) => c.name === name);
}

async function expectSignedOut(page: Page) {
  await page.goto("/learn");
  await expect(page.getByRole("heading", { name: "Sign in as the fictional learner" })).toBeVisible();
  await expect(page.getByRole("heading", { name: /Welcome back/ })).toHaveCount(0);
}

async function expectRejectedCallback(page: Page) {
  await expect(page).toHaveURL(/\/auth\/error\?code=callback_rejected$/);
  await expect(page.getByRole("heading", { name: "Sign-in could not be completed" })).toBeVisible();
  // The generic page never echoes provider detail or the rejected parameters.
  await expect(page.locator("body")).not.toContainText(/access_denied|invalid_grant|state=|code=/);
}

// Starts a login and returns the state the app generated for it, read from the provider's
// authorize URL the browser was sent to. The transaction cookie is now set in the context.
async function beginLogin(page: Page, returnTo?: string): Promise<string> {
  await page.goto("/api/auth/login" + (returnTo ? `?returnTo=${encodeURIComponent(returnTo)}` : ""));
  await expect(page.getByRole("heading", { name: "Choose a demo identity" })).toBeVisible();
  const state = new URL(page.url()).searchParams.get("state");
  expect(state, "authorize request carries state").toBeTruthy();
  return state as string;
}

test.describe("session cookie", () => {
  test("a tampered session cookie is treated as signed out", async ({ page, context }) => {
    await setCookie(context, SESSION_COOKIE, "not.a.valid.cookie");
    await expectSignedOut(page);
  });

  test("a session sealed under a different secret is treated as signed out", async ({ page, context }) => {
    const future = Math.floor(Date.now() / 1000) + 600;
    await setCookie(
      context,
      SESSION_COOKIE,
      seal({ accessToken: "forged", subject: "demo|admin", expiresAt: future }, "attacker-secret-of-at-least-32-chars"),
    );
    await expectSignedOut(page);
    await page.goto("/admin/compliance");
    await expect(page.getByRole("heading", { name: "Participation compliance" })).toHaveCount(0);
  });

  test("an expired session is treated as signed out even though it is validly sealed", async ({ page, context }) => {
    const past = Math.floor(Date.now() / 1000) - 60;
    await setCookie(context, SESSION_COOKIE, seal({ accessToken: "stale", subject: "demo|learner", expiresAt: past }));
    await expectSignedOut(page);
  });

  test("a validly sealed session with a missing token is treated as signed out", async ({ page, context }) => {
    const future = Math.floor(Date.now() / 1000) + 600;
    await setCookie(context, SESSION_COOKIE, seal({ subject: "demo|learner", expiresAt: future }));
    await expectSignedOut(page);
  });

  test("a real session cookie is HttpOnly and SameSite=Lax, and logout requires POST", async ({ page, context }) => {
    await beginLogin(page, "/learn");
    await page.getByRole("button", { name: "Continue as Alex Coach (learner)" }).click();
    await expect(page.getByRole("heading", { name: /Welcome back, Alex Coach/ })).toBeVisible();

    const session = await cookie(context, SESSION_COOKIE);
    expect(session?.httpOnly).toBe(true);
    expect(session?.sameSite).toBe("Lax");
    expect(await cookie(context, TRANSACTION_COOKIE), "transaction cookie is consumed by the callback").toBeUndefined();

    // A cross-site GET (an <img> or a link) must not be able to log the user out.
    const logoutByGet = await page.request.get("/api/auth/logout", { maxRedirects: 0 });
    expect(logoutByGet.status()).toBe(405);
    await page.goto("/learn");
    await expect(page.getByRole("heading", { name: /Welcome back, Alex Coach/ })).toBeVisible();
  });
});

test.describe("callback", () => {
  test("a callback with no login transaction is rejected", async ({ page, context }) => {
    await page.goto("/api/auth/callback?code=anything&state=anything");
    await expectRejectedCallback(page);
    expect(await cookie(context, SESSION_COOKIE)).toBeUndefined();
  });

  test("a callback whose state does not match the transaction is rejected", async ({ page, context }) => {
    await beginLogin(page);
    await page.goto("/api/auth/callback?code=anything&state=not-the-issued-state");
    await expectRejectedCallback(page);
    expect(await cookie(context, SESSION_COOKIE)).toBeUndefined();
    // The transaction is single-use: it was consumed by the rejected attempt.
    expect(await cookie(context, TRANSACTION_COOKIE)).toBeUndefined();
  });

  test("a provider error response with the right state is rejected without a session", async ({ page, context }) => {
    const state = await beginLogin(page);
    await page.goto(
      `/api/auth/callback?error=access_denied&error_description=User%20cancelled&state=${encodeURIComponent(state)}`,
    );
    await expectRejectedCallback(page);
    expect(await cookie(context, SESSION_COOKIE)).toBeUndefined();
  });

  test("a forged authorization code with the right state is rejected at the token exchange", async ({ page, context }) => {
    const state = await beginLogin(page);
    await page.goto(`/api/auth/callback?code=forged-code&state=${encodeURIComponent(state)}`);
    await expectRejectedCallback(page);
    expect(await cookie(context, SESSION_COOKIE)).toBeUndefined();
  });

  test("an expired login transaction is rejected even when state matches", async ({ page, context }) => {
    const state = "expired-transaction-state";
    await setCookie(
      context,
      TRANSACTION_COOKIE,
      seal({ state, nonce: "n", verifier: "v".repeat(43), returnTo: "/learn", expiresAt: Math.floor(Date.now() / 1000) - 1 }),
    );
    await page.goto(`/api/auth/callback?code=anything&state=${state}`);
    await expectRejectedCallback(page);
    expect(await cookie(context, SESSION_COOKIE)).toBeUndefined();
  });

  test("a completed callback cannot be replayed", async ({ page, context }) => {
    await beginLogin(page, "/learn");
    const callback = page.waitForRequest((request) => request.url().includes("/api/auth/callback?"));
    await page.getByRole("button", { name: "Continue as Alex Coach (learner)" }).click();
    const callbackURL = (await callback).url();
    await expect(page.getByRole("heading", { name: /Welcome back, Alex Coach/ })).toBeVisible();

    // Same code, same state, second time: the transaction is gone and the code is spent.
    await page.goto(callbackURL);
    await expectRejectedCallback(page);
    // The replay must not have minted a second session or disturbed the first.
    const sessions = (await context.cookies()).filter((c) => c.name === SESSION_COOKIE);
    expect(sessions).toHaveLength(1);
  });
});

test.describe("returnTo", () => {
  for (const hostile of ["https://evil.example/phish", "//evil.example/phish", "/admin/../learn", "javascript:alert(1)"]) {
    test(`login with returnTo=${hostile} lands on the home page after sign-in`, async ({ page }) => {
      await beginLogin(page, hostile);
      await page.getByRole("button", { name: "Continue as Alex Coach (learner)" }).click();
      await expect(page).toHaveURL(new RegExp(`^${baseURL().replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/?$`));
      await expect(page.getByRole("heading", { name: /Learning progress and participation eligibility/ })).toBeVisible();
    });
  }

  test("login with an allow-listed returnTo lands there", async ({ page }) => {
    await beginLogin(page, "/learn");
    await page.getByRole("button", { name: "Continue as Alex Coach (learner)" }).click();
    await expect(page).toHaveURL(/\/learn$/);
  });
});
