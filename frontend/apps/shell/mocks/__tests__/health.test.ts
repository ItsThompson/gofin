import { describe, it, expect } from "vitest";
import type { HealthScore, HealthScoreResponse } from "@gofin/core";
import { computeMockHealthScore, mockHealthScore } from "../data";
import { healthScoreHandlers } from "../handlers/health-score";
import { resolveMockRequest } from "./drive";

describe("computeMockHealthScore", () => {
  it("produces a canonical HealthScore: HealthBand, known component keys, no configureBudget", () => {
    const score: HealthScore = computeMockHealthScore();

    expect(["green", "amber", "red"]).toContain(score.band);
    expect(score.total).toBeGreaterThanOrEqual(0);
    expect(score.total).toBeLessThanOrEqual(100);
    // The score variant must not carry the configure-budget flag; that state is
    // a separate variant of HealthScoreResponse, not a field on HealthScore.
    expect(score).not.toHaveProperty("configureBudget");

    expect(score.components.map((component) => component.key)).toEqual([
      "savings_achievement",
      "budget_adherence",
      "allocation_balance",
      "spending_stability",
    ]);
    expect(score.insight.driver).not.toBe("");
  });

  it("stays faithful to the ported Go formula for the fixed mock inputs", () => {
    // Locks the port against mockPeriod ($3,000, 50/30/20) + mockExpenses so a
    // future refactor of the formula cannot silently drift the mock card.
    expect(mockHealthScore.total).toBe(63);
    expect(mockHealthScore.band).toBe("amber");
  });

  it("accepts the configure-budget variant in the response contract", () => {
    const configurePrompt: HealthScoreResponse = { healthScore: { configureBudget: true } };
    expect(configurePrompt.healthScore).toEqual({ configureBudget: true });
  });
});

describe("GET /api/finance/health-score", () => {
  it("returns a HealthScoreResponse wrapping the score variant", async () => {
    const res = await resolveMockRequest(
      healthScoreHandlers,
      "/api/finance/health-score",
    );
    const body = (await res.json()) as HealthScoreResponse;

    expect(body.healthScore).toBeDefined();
    const healthScore = body.healthScore as HealthScore;
    expect(healthScore.components).toHaveLength(4);
    expect(["green", "amber", "red"]).toContain(healthScore.band);
  });
});
