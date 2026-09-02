import { expect, test } from "@playwright/test";

test.skip(process.env.PLAYWRIGHT_OIDC !== "1", "requires the local OIDC Compose overlay");

test("learner and administrator complete explicit OIDC journeys", async ({ page }) => {
  await page.goto("/learn");
  await expect(page.getByRole("heading", { name: "Sign in as the fictional learner" })).toBeVisible();

  await page.getByRole("link", { name: "Sign in to the learner demo" }).click();
  await expect(page.getByRole("heading", { name: "Choose a demo identity" })).toBeVisible();
  await page.getByRole("button", { name: "Continue as Alex Coach (learner)" }).click();
  await expect(page).toHaveURL(/\/learn$/);
  await expect(page.getByRole("heading", { name: /Welcome back, Alex Coach/ })).toBeVisible();

  const enroll = page.getByRole("button", { name: "Enroll in course" });
  if (await enroll.isVisible()) await enroll.click();
  await expect(page.getByRole("heading", { name: "Grassroots Match-Day Safety" })).toBeVisible();
  await page.getByRole("button", { name: "Mark complete" }).click();
  await expect(page.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "33");

  await page.goto("/admin/compliance");
  await expect(page.getByRole("heading", { name: "This identity is not an administrator" })).toBeVisible();
  await page.getByRole("button", { name: "Sign out and switch identity" }).click();

  await page.goto("/admin/compliance");
  await page.getByRole("link", { name: "Sign in to the administrator demo" }).click();
  await page.getByRole("button", { name: "Continue as Casey Admin (administrator)" }).click();
  await expect(page).toHaveURL(/\/admin\/compliance$/);
  await expect(page.getByRole("heading", { name: "Participation compliance" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Casey Admin" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
});
