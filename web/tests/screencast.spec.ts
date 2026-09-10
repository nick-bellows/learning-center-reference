import { expect, test, type Page } from "@playwright/test";

// Records the recruiter walkthrough shown in the README: the guided landing page, the learner
// journey through the local OIDC fixture (sign in, enroll, complete two lessons), the
// authorization boundary (a learner is refused the administrator view), and the administrator
// roster. It is a recording aid, not a test gate: it only runs when PLAYWRIGHT_SCREENCAST=1
// and needs the OIDC Compose overlay. scripts/record-screencast.ps1 wraps it and renders the
// captured video to docs/assets/recruiter-walkthrough.gif with ffmpeg.
test.skip(process.env.PLAYWRIGHT_SCREENCAST !== "1", "recording aid; set PLAYWRIGHT_SCREENCAST=1");

const SIZE = { width: 1280, height: 720 };
test.use({ viewport: SIZE, video: { mode: "on", size: SIZE }, colorScheme: "light" });

// Pauses are what make the recording readable at GIF frame rates; they are not waits for state.
const beat = (page: Page, ms = 1400) => page.waitForTimeout(ms);

async function settle(page: Page, heading: RegExp | string) {
  await expect(page.getByRole("heading", { name: heading })).toBeVisible();
  await beat(page);
}

test("recruiter walkthrough", async ({ page }) => {
  // 1. The landing page states the two workflows and links behaviour to code.
  await page.goto("/");
  await settle(page, /Learning progress and participation eligibility/);
  await page.mouse.wheel(0, 700);
  await beat(page, 1800);
  await page.mouse.wheel(0, -700);
  await beat(page, 800);

  // 2. Learner journey: real redirect to the local OIDC fixture, back with a session.
  await page.goto("/learn");
  await settle(page, "Sign in as the fictional learner");
  await page.getByRole("link", { name: "Sign in to the learner demo" }).click();
  await settle(page, "Choose a demo identity");
  await page.getByRole("button", { name: "Continue as Alex Coach (learner)" }).click();
  await settle(page, /Welcome back, Alex Coach/);

  const enroll = page.getByRole("button", { name: "Enroll in course" });
  if (await enroll.isVisible()) {
    await enroll.click();
    await settle(page, "Grassroots Match-Day Safety");
  }
  for (const percent of ["33", "66"]) {
    await page.getByRole("button", { name: "Mark complete" }).first().click();
    await expect(page.getByRole("progressbar")).toHaveAttribute("aria-valuenow", percent);
    await beat(page, 1600);
  }

  // 3. Authorization boundary: the learner's identity is refused the administrator view.
  await page.goto("/admin/compliance");
  await settle(page, "This identity is not an administrator");
  await page.getByRole("button", { name: "Sign out and switch identity" }).click();
  await beat(page, 600);

  // 4. Administrator roster with derived eligibility.
  await page.goto("/admin/compliance");
  await settle(page, "Sign in as the fictional administrator");
  await page.getByRole("link", { name: "Sign in to the administrator demo" }).click();
  await settle(page, "Choose a demo identity");
  await page.getByRole("button", { name: "Continue as Casey Admin (administrator)" }).click();
  await settle(page, "Participation compliance");
  await expect(page.getByRole("cell", { name: "Riley Referee (synthetic)" })).toBeVisible();
  await page.mouse.wheel(0, 460);
  await beat(page, 3000);
});
