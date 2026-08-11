import { test, expect, type Locator, type Page } from "@playwright/test";
import {
  registerAndOnboard,
  confirmNewPeriod,
  logExpense,
} from "../helpers/auth.js";

const VIEWPORT_HEIGHT = 900;

test.describe("Dashboard Outline Rail", () => {
  test("stays hidden below 2xl and clears the dashboard cards at 2xl and above", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: VIEWPORT_HEIGHT });

    // Setup: an active period with one expense, so the dashboard renders
    // enough sections for the outline to collect items.
    await registerAndOnboard(page, { budget: "2000" });
    await confirmNewPeriod(page);
    await logExpense(page, {
      name: "Outline Coffee",
      amount: "12.50",
      type: "essentials",
    });

    await page.goto("/dashboard");
    await expect(page.getByText("Total Budget")).toBeVisible();

    const outlineRail = page.getByRole("navigation", {
      name: "Dashboard sections",
    });

    // Step 1: At 1440px the space beside the cards is too narrow, so the rail is off.
    await expect(outlineRail).toBeHidden();

    // Step 2: From 1536px up the rail is on and must sit beside the cards, not on top of them.
    for (const width of [1536, 1920]) {
      await page.setViewportSize({ width, height: VIEWPORT_HEIGHT });
      await expect(outlineRail).toBeVisible();
      await expectRailClearOfCards(page, outlineRail);
    }
  });
});

/**
 * The rail is fixed at the top of the viewport and the cards fill the same
 * vertical band, so horizontal separation is what keeps them from overlapping:
 * the rail's left edge must start at or after the rightmost card's right edge.
 */
async function expectRailClearOfCards(page: Page, rail: Locator) {
  const railBox = await rail.boundingBox();
  if (!railBox) throw new Error("Outline rail is visible but has no bounding box");

  const cards = page.locator('[data-slot="card"]');
  expect(await cards.count()).toBeGreaterThan(0);

  const rightmostCardEdge = await cards.evaluateAll((elements) =>
    Math.max(...elements.map((element) => element.getBoundingClientRect().right)),
  );

  expect(railBox.x).toBeGreaterThanOrEqual(rightmostCardEdge);
}
