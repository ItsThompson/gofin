import type {
  HealthBand,
  HealthComponent,
  HealthComponentKey,
  HealthInsight,
  HealthScore,
  HealthScoreTrendPoint,
} from "@gofin/core";
import { currentYear, currentMonth } from "./foundation";
import { mockPeriod } from "./periods";
import { mockExpenses } from "./expenses";

/**
 * A mock health component whose key is one of the known component keys. The
 * canonical `HealthComponent.key` is an open union; narrowing it here keeps the
 * derived insight `driver` assignable to `HealthInsight["driver"]`.
 */
type ScoredComponent = HealthComponent & { key: HealthComponentKey };

function formatHealthMoney(cents: number): string {
  const negative = cents < 0;
  const abs = Math.abs(cents);
  const dollars = Math.floor(abs / 100);
  const remainder = abs % 100;
  let out = "$" + dollars.toLocaleString("en-US");
  if (remainder !== 0) out += "." + String(remainder).padStart(2, "0");
  return negative ? "-" + out : out;
}

function clampRound(value: number, max: number): number {
  return Math.min(max, Math.max(0, Math.round(value)));
}

// Weights and stability tuning for health-score formula version 2: mirror the
// authoritative Go formula in services/finance/internal/service/healthscore.go.
// dev-mock has no backend, so this port must stay faithful to keep the card
// representative.
const HEALTH_BASE_WEIGHTS = { savings: 25, budget: 25, allocation: 30, stability: 20 };
const STABILITY_MIN_MONTHS = 3;
const STABILITY_COV_CAP = 1.0;

// Synthetic recent closed-month desires totals (cents) so the stability
// sub-score computes in dev-mock. Real values come from the backend.
const mockDesiresWindow = [21000, 19200, 22500, 18700, 20100];

type HealthWeights = {
  savings: number;
  budget: number;
  allocation: number;
  stability: number;
  maxSavings: number;
  maxBudget: number;
  maxAllocation: number;
  maxStability: number;
};

// resolveMockWeights renormalizes the base weights over the present set by
// division, rounding the remainder into allocation, exactly like resolveWeights
// on the backend.
function resolveMockWeights(savingsPresent: boolean, stabilityPresent: boolean): HealthWeights {
  const base = HEALTH_BASE_WEIGHTS;
  let denom = base.budget + base.allocation;
  if (savingsPresent) denom += base.savings;
  if (stabilityPresent) denom += base.stability;
  const eff = (weight: number) => (100 * weight) / denom;

  const savings = savingsPresent ? eff(base.savings) : 0;
  const budget = eff(base.budget);
  const allocation = eff(base.allocation);
  const stability = stabilityPresent ? eff(base.stability) : 0;

  const maxSavings = savingsPresent ? Math.round(savings) : 0;
  const maxBudget = Math.round(budget);
  const maxStability = stabilityPresent ? Math.round(stability) : 0;
  const maxAllocation = 100 - (maxSavings + maxBudget + maxStability);

  return { savings, budget, allocation, stability, maxSavings, maxBudget, maxAllocation, maxStability };
}

// mockStability ports stabilityComponent: full marks for a zero mean, otherwise
// weight * clamp(1 - CoV / cap, 0, 1) with a sample (n-1) standard deviation.
function mockStability(window: number[], weight: number): { score: number; detail: string } {
  const mean = window.reduce((sum, value) => sum + value, 0) / window.length;
  if (mean === 0) return { score: weight, detail: "Desires spend held steady month to month" };

  const variance =
    window.reduce((sum, value) => sum + (value - mean) ** 2, 0) / (window.length - 1);
  const cov = Math.sqrt(variance) / mean;
  const ratio = Math.min(1, Math.max(0, 1 - cov / STABILITY_COV_CAP));
  const pct = Math.round(cov * 100);
  const detail =
    pct <= 0
      ? "Desires spend held steady month to month"
      : `Desires spend varied ~${pct}% month to month`;
  return { score: weight * ratio, detail };
}

export function computeMockHealthScore(): HealthScore {
  const budget = mockPeriod.budgetAmount;
  const savingsTarget = Math.floor((budget * mockPeriod.savingsPercent) / 100);
  const combinedTarget = Math.floor(
    (budget * (mockPeriod.essentialsPercent + mockPeriod.desiresPercent)) / 100,
  );

  const sumByType = (type: string) =>
    mockExpenses
      .filter((expense) => expense.expenseType === type)
      .reduce((sum, expense) => sum + expense.transactionAmount, 0);
  const essentialsActual = sumByType("essentials");
  const desiresActual = sumByType("desires");
  const savingsActual = sumByType("savings");
  const edActual = essentialsActual + desiresActual;

  const savingsDropped = savingsTarget === 0;
  const stabilityPresent = mockDesiresWindow.length >= STABILITY_MIN_MONTHS;
  const w = resolveMockWeights(!savingsDropped, stabilityPresent);

  const components: ScoredComponent[] = [];

  if (!savingsDropped) {
    const ratio = Math.min(1, savingsActual / savingsTarget);
    components.push({
      key: "savings_achievement",
      score: clampRound(w.savings * ratio, w.maxSavings),
      max: w.maxSavings,
      detail: `Saved ${formatHealthMoney(savingsActual)} of ${formatHealthMoney(savingsTarget)} target`,
    });
  }

  const budgetRatio = combinedTarget === 0 ? (edActual === 0 ? 0 : 2) : edActual / combinedTarget;
  const budgetFactor =
    budgetRatio <= 1 ? 1 : budgetRatio >= 1.5 ? 0 : (1.5 - budgetRatio) / 0.5;
  components.push({
    key: "budget_adherence",
    score: clampRound(w.budget * budgetFactor, w.maxBudget),
    max: w.maxBudget,
    detail: `Spent ${formatHealthMoney(edActual)} of ${formatHealthMoney(combinedTarget)} plan`,
  });

  const spendDenom = savingsDropped ? edActual : edActual + savingsActual;
  const percentSum = savingsDropped
    ? mockPeriod.essentialsPercent + mockPeriod.desiresPercent
    : 100;
  let allocScore = w.allocation;
  let allocDetail = "Balanced across categories";
  const devs: { label: string; dev: number }[] = [];
  if (spendDenom > 0) {
    const devE = essentialsActual / spendDenom - mockPeriod.essentialsPercent / percentSum;
    const devD = desiresActual / spendDenom - mockPeriod.desiresPercent / percentSum;
    let wdev = 0.5 * Math.abs(devE) + (devD > 0 ? 1 : 0.5) * Math.abs(devD);
    devs.push({ label: "Essentials", dev: devE }, { label: "Desires", dev: devD });
    if (!savingsDropped) {
      const devS = savingsActual / spendDenom - mockPeriod.savingsPercent / percentSum;
      wdev += (devS < 0 ? 1 : 0.5) * Math.abs(devS);
      devs.push({ label: "Savings", dev: devS });
    }
    allocScore = w.allocation * (1 - Math.min(1, wdev));
    const over = devs.filter((d) => d.dev > 0).sort((a, b) => b.dev - a.dev)[0];
    if (over && Math.round(over.dev * 100) > 0) {
      allocDetail = `${over.label} ${Math.round(over.dev * 100)} pts over target share`;
    }
  }
  components.push({
    key: "allocation_balance",
    score: clampRound(allocScore, w.maxAllocation),
    max: w.maxAllocation,
    detail: allocDetail,
  });

  if (stabilityPresent) {
    const stability = mockStability(mockDesiresWindow, w.stability);
    components.push({
      key: "spending_stability",
      score: clampRound(stability.score, w.maxStability),
      max: w.maxStability,
      detail: stability.detail,
    });
  }

  const total = components.reduce((sum, component) => sum + component.score, 0);
  const band: HealthBand = total >= 80 ? "green" : total >= 55 ? "amber" : "red";

  const driver = components.reduce((lowest, component) =>
    component.score < lowest.score ? component : lowest,
  );
  const insight = buildMockInsight(driver, {
    savingsGap: savingsTarget - savingsActual,
    overspend: edActual - combinedTarget,
    devs,
    maxSavings: w.maxSavings,
    maxBudget: w.maxBudget,
    maxAllocation: w.maxAllocation,
    maxStability: w.maxStability,
  });

  return {
    year: currentYear,
    month: currentMonth,
    total,
    band,
    provisional: true,
    formulaVersion: 2,
    reportingCurrency: "USD",
    components,
    insight,
  };
}

function buildMockInsight(
  driver: ScoredComponent,
  ctx: {
    savingsGap: number;
    overspend: number;
    devs: { label: string; dev: number }[];
    maxSavings: number;
    maxBudget: number;
    maxAllocation: number;
    maxStability: number;
  },
): HealthInsight {
  if (driver.key === "savings_achievement") {
    return {
      summary: "Savings is the softest score this month.",
      driver: driver.key,
      nudge: `Move an extra ${formatHealthMoney(ctx.savingsGap)} to savings to reach your target and lift your score about ${ctx.maxSavings - driver.score} points.`,
    };
  }
  if (driver.key === "budget_adherence") {
    return {
      summary: "Budget adherence is the softest score this month.",
      driver: driver.key,
      nudge:
        ctx.overspend > 0
          ? `Trim about ${formatHealthMoney(ctx.overspend)} from essentials and desires to get back to plan and lift your score about ${ctx.maxBudget - driver.score} points.`
          : "Keep essentials and desires within your plan to lift this score.",
    };
  }
  if (driver.key === "spending_stability") {
    return {
      summary: "Spending stability is the softest score this month.",
      driver: driver.key,
      nudge: `Steadier discretionary spending month to month could lift your score about ${ctx.maxStability - driver.score} points.`,
    };
  }
  const over = ctx.devs.filter((d) => d.dev > 0).sort((a, b) => b.dev - a.dev)[0];
  const under = ctx.devs.filter((d) => d.dev < 0).sort((a, b) => a.dev - b.dev)[0];
  return {
    summary: "Your category balance is the softest score this month.",
    driver: driver.key,
    nudge:
      over && under
        ? `${over.label} is running ${Math.round(over.dev * 100)} pts over its target share. Shifting spend toward ${under.label} could recover up to ${ctx.maxAllocation - driver.score} points.`
        : `Rebalancing your categories could recover up to ${ctx.maxAllocation - driver.score} points.`,
  };
}

export const mockHealthScore: HealthScore = computeMockHealthScore();

// Sample historical totals for the sparkline (oldest first). The current month
// uses the real computed total and is flagged provisional as the last point.
const MOCK_TREND_TOTALS = [55, 61, 58, 66, 72, 69, 74, 70, 63, 68, 71];

export function computeMockHealthScoreTrend(months: number): HealthScoreTrendPoint[] {
  // Match the Go service clamp policy: default 6, cap 12.
  const requested = Number.isFinite(months) ? months : 6;
  const count = requested < 1 ? 6 : Math.min(requested, 12);
  const bandFor = (total: number): HealthBand =>
    total >= 80 ? "green" : total >= 55 ? "amber" : "red";

  const points: HealthScoreTrendPoint[] = [];
  for (let offset = count - 1; offset >= 0; offset--) {
    let year = currentYear;
    let month = currentMonth - offset;
    while (month <= 0) {
      month += 12;
      year -= 1;
    }
    const provisional = offset === 0;
    const total = provisional
      ? mockHealthScore.total
      : MOCK_TREND_TOTALS[(count - 1 - offset) % MOCK_TREND_TOTALS.length];
    points.push({ year, month, total, band: bandFor(total), provisional, formulaVersion: 2, reportingCurrency: "USD" });
  }
  return points;
}

export const mockHealthScoreTrend: HealthScoreTrendPoint[] = computeMockHealthScoreTrend(6);
