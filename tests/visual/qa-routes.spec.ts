import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { test, expect } from "@playwright/test";

const routes = [
  ["normal", "/?qa=normal"],
  ["loading", "/?qa=loading"],
  ["error", "/?qa=error"],
  ["empty", "/?qa=empty"],
  ["search", "/?qa=search"],
  ["search-account", "/?qa=search-account"],
  ["search-empty", "/?qa=search-empty"],
  ["multiselect", "/?qa=multiselect"],
  ["compose", "/?qa=compose"],
  ["remote-images-shown", "/?qa=remote-images-shown"],
  ["quoted-expanded", "/?qa=quoted-expanded"],
  ["long-overflow", "/?qa=long-overflow"],
  ["many-attachments", "/?qa=many-attachments"],
  ["empty-custom-folder", "/?qa=empty-custom-folder"],
  ["nested-tree", "/?qa=nested-tree"],
  ["mobile-reading-attachments", "/?qa=mobile-reading-attachments"],
  ["compose-cc-bcc", "/?qa=compose-cc-bcc"],
] as const;

const viewports = [
  { label: "desktop", width: 1440, height: 900 },
  { label: "mobile", width: 375, height: 812 },
] as const;

const outputDir = join("docs", "qa-screenshots", new Date().toISOString().slice(0, 10));

for (const viewport of viewports) {
  test.describe(`${viewport.label} QA states`, () => {
    test.beforeAll(() => mkdirSync(outputDir, { recursive: true }));

    for (const [name, path] of routes) {
      test(`${viewport.label}-${name}`, async ({ page }) => {
        await page.setViewportSize({ width: viewport.width, height: viewport.height });
        await page.goto(path);
        await expect(page.locator("body")).toBeVisible();
        await page.waitForTimeout(150);
        await page.screenshot({
          path: join(outputDir, `${viewport.label}-${name}.png`),
          fullPage: true,
        });
      });
    }
  });
}
