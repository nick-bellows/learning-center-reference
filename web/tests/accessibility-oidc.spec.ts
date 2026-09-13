import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

// accessibility.spec.ts runs axe against the demo-mode stack, where every page renders its
// signed-in state. This suite covers the states only the OIDC overlay produces: the
// signed-out learner and administrator prompts, the "not an administrator" refusal, the
// generic sign-in error page, the 404 page, and a phone-width viewport. It runs in the
// oidc-e2e CI job alongside the auth suites.
test.skip(process.env.PLAYWRIGHT_OIDC !== "1", "requires the local OIDC Compose overlay");

async function expectNoViolations(page: Page) {
  await expect(page.locator("h1")).toBeVisible();
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();
  expect(results.violations).toEqual([]);
}

test("signed-out learner prompt", async ({ page }) => {
  await page.goto("/learn");
  await expect(page.getByRole("heading", { name: "Sign in as the fictional learner" })).toBeVisible();
  await expectNoViolations(page);
});

test("signed-out administrator prompt", async ({ page }) => {
  await page.goto("/admin/compliance");
  await expect(page.getByRole("heading", { name: "Sign in as the fictional administrator" })).toBeVisible();
  await expectNoViolations(page);
});

test("learner refused the administrator view", async ({ page }) => {
  await page.goto("/api/auth/login?returnTo=/learn");
  await page.getByRole("button", { name: "Continue as Alex Coach (learner)" }).click();
  await expect(page.getByRole("heading", { name: /Welcome back, Alex Coach/ })).toBeVisible();
  await page.goto("/admin/compliance");
  await expect(page.getByRole("heading", { name: "This identity is not an administrator" })).toBeVisible();
  await expectNoViolations(page);
});

test("sign-in error page", async ({ page }) => {
  await page.goto("/auth/error?code=callback_rejected");
  await expectNoViolations(page);
});

test("404 page keeps the site chrome and is accessible", async ({ page }) => {
  const response = await page.goto("/this-route-does-not-exist");
  expect(response?.status()).toBe(404);
  await expect(page.getByRole("heading", { name: "There is no page at this address" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Primary navigation" })).toBeVisible();
  await expectNoViolations(page);
});

test.describe("phone width", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  for (const path of ["/", "/members", "/learn"]) {
    test(`${path} at 390px`, async ({ page }) => {
      await page.goto(path);
      await expectNoViolations(page);
      // No horizontal scrolling: the page body must fit the viewport.
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(overflow, "page should not scroll horizontally at phone width").toBe(false);
    });
  }
});
