import { expect, test } from "@playwright/test";

test("run history lists what the API reports", async ({ page, request }) => {
  const response = await request.get("/api/v1/runs");
  expect(response.ok()).toBeTruthy();
  const { items } = await response.json();
  expect(items.length).toBeGreaterThan(0);

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Runs" })).toBeVisible();

  // Every run the API returned is on the page.
  for (const run of items) {
    await expect(page.getByRole("cell", { name: `#${run.id}`, exact: true })).toBeVisible();
  }
});

test("a run detail page explains why a line was rejected", async ({ page, request }) => {
  const response = await request.get("/api/v1/runs");
  const { items } = await response.json();
  const failing = items.find((run: { failed_count: number }) => run.failed_count > 0);
  test.skip(!failing, "no run with rejected lines in this fixture");

  await page.goto(`/#/runs/${failing.id}`);
  await expect(page.getByRole("heading", { name: `Run #${failing.id}` })).toBeVisible();
  await expect(page.getByText("Events")).toBeVisible();
  // The reason, not just the count, has to be reachable from the UI.
  await expect(page.getByText(/rejected/i).first()).toBeVisible();
});

test("settings show each value's origin and flag environment overrides", async ({ page }) => {
  await page.goto("/#/settings");
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByText("server.addr")).toBeVisible();

  // The fixture server runs with MUXIO_LOG_LEVEL set.
  await expect(page.getByText("from environment")).toBeVisible();
  await expect(page.getByText(/Overridden by an environment variable/)).toBeVisible();
});

test("navigation moves between runs and settings", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

  await page.getByRole("button", { name: "Runs" }).click();
  await expect(page.getByRole("heading", { name: "Runs" })).toBeVisible();
});
