import { describe, it, expect } from "vitest";
import {
  buildUser,
  buildPeriod,
  buildExpense,
  buildTag,
  buildPeriodSummary,
  buildDefaults,
  buildProRataSchedule,
} from "../factories";

describe("buildUser", () => {
  it("returns a valid User with all required fields", () => {
    const user = buildUser();

    expect(user.id).toMatch(/^user-\d+$/);
    expect(user.username).toBe("testuser");
    expect(user.email).toBe("test@example.com");
    expect(user.role).toBe("user");
    expect(user.currency).toBe("USD");
    expect(user.hasCompletedOnboarding).toBe(true);
    expect(user.createdAt).toBe("2026-01-15T00:00:00Z");
  });

  it("produces unique incrementing IDs across calls", () => {
    const user1 = buildUser();
    const user2 = buildUser();

    expect(user1.id).not.toBe(user2.id);
    const id1Num = parseInt(user1.id.split("-")[1]);
    const id2Num = parseInt(user2.id.split("-")[1]);
    expect(id2Num).toBe(id1Num + 1);
  });

  it("accepts Partial<User> overrides", () => {
    const user = buildUser({ username: "alice", role: "admin" });

    expect(user.username).toBe("alice");
    expect(user.role).toBe("admin");
    expect(user.email).toBe("test@example.com");
  });
});

describe("buildPeriod", () => {
  it("returns a valid BudgetPeriod with all required fields", () => {
    const period = buildPeriod();

    expect(period.id).toMatch(/^period-\d+$/);
    expect(period.userId).toBe("user-1");
    expect(period.year).toBe(2026);
    expect(period.month).toBe(1);
    expect(period.budgetAmount).toBe(300000);
    expect(period.essentialsPercent).toBe(50);
    expect(period.desiresPercent).toBe(30);
    expect(period.savingsPercent).toBe(20);
    expect(period.createdAt).toBe("2026-01-15T00:00:00Z");
    expect(period.updatedAt).toBe("2026-01-15T00:00:00Z");
  });

  it("produces unique incrementing IDs across calls", () => {
    const period1 = buildPeriod();
    const period2 = buildPeriod();

    expect(period1.id).not.toBe(period2.id);
  });

  it("accepts partial overrides", () => {
    const period = buildPeriod({ year: 2025, month: 12, budgetAmount: 500000 });

    expect(period.year).toBe(2025);
    expect(period.month).toBe(12);
    expect(period.budgetAmount).toBe(500000);
    expect(period.essentialsPercent).toBe(50);
  });
});

describe("buildExpense", () => {
  it("returns a valid Expense with all required fields", () => {
    const expense = buildExpense();

    expect(expense.id).toMatch(/^expense-\d+$/);
    expect(expense.userId).toBe("user-1");
    expect(expense.name).toBe("Test Expense");
    expect(expense.amount).toBe(5000);
    expect(expense.currency).toBe("USD");
    expect(expense.expenseType).toBe("essentials");
    expect(expense.tagId).toBe("tag-1");
    expect(expense.expenseDate).toBe("2026-01-15");
    expect(expense.periodYear).toBe(2026);
    expect(expense.periodMonth).toBe(1);
    expect(expense.status).toBe("active");
    expect(expense.isProRata).toBe(false);
    expect(expense.createdAt).toBe("2026-01-15T00:00:00Z");
  });

  it("produces unique incrementing IDs across calls", () => {
    const expense1 = buildExpense();
    const expense2 = buildExpense();

    expect(expense1.id).not.toBe(expense2.id);
  });

  it("accepts partial overrides", () => {
    const expense = buildExpense({
      name: "Coffee",
      amount: 450,
      expenseType: "desires",
    });

    expect(expense.name).toBe("Coffee");
    expect(expense.amount).toBe(450);
    expect(expense.expenseType).toBe("desires");
    expect(expense.userId).toBe("user-1");
  });
});

describe("buildTag", () => {
  it("returns a valid Tag with all required fields", () => {
    const tag = buildTag();

    expect(tag.id).toMatch(/^tag-\d+$/);
    expect(tag.name).toBe("Food");
    expect(tag.isDefault).toBe(false);
    expect(tag.createdAt).toBe("2026-01-15T00:00:00Z");
    expect(tag.updatedAt).toBe("2026-01-15T00:00:00Z");
  });

  it("produces unique incrementing IDs across calls", () => {
    const tag1 = buildTag();
    const tag2 = buildTag();

    expect(tag1.id).not.toBe(tag2.id);
  });

  it("accepts partial overrides", () => {
    const tag = buildTag({ name: "Transport", isDefault: true });

    expect(tag.name).toBe("Transport");
    expect(tag.isDefault).toBe(true);
  });
});

describe("buildPeriodSummary", () => {
  it("returns an internally consistent summary with derived allocations", () => {
    const summary = buildPeriodSummary();

    expect(summary.totalBudget).toBe(300000);
    expect(summary.essentials.allocated).toBe(150000); // 300000 * 50 / 100
    expect(summary.desires.allocated).toBe(90000);     // 300000 * 30 / 100
    expect(summary.savings.allocated).toBe(60000);     // 300000 * 20 / 100
  });

  it("derives allocations from overridden totalBudget", () => {
    const summary = buildPeriodSummary({ totalBudget: 500000 });

    expect(summary.essentials.allocated).toBe(250000); // 500000 * 50 / 100
    expect(summary.desires.allocated).toBe(150000);    // 500000 * 30 / 100
    expect(summary.savings.allocated).toBe(100000);    // 500000 * 20 / 100
  });

  it("defaults totalSpent to 0 with correct remaining", () => {
    const summary = buildPeriodSummary();

    expect(summary.totalSpent).toBe(0);
    expect(summary.remaining).toBe(300000);
  });

  it("allows overriding nested category summaries", () => {
    const summary = buildPeriodSummary({
      essentials: { allocated: 150000, spent: 50000, remaining: 100000, percentUsed: 33.33 },
    });

    expect(summary.essentials.spent).toBe(50000);
    expect(summary.essentials.remaining).toBe(100000);
  });

  it("calculates dailySpendRate from totalSpent and daysElapsed", () => {
    const summary = buildPeriodSummary({ totalSpent: 30000, daysElapsed: 10 });

    expect(summary.dailySpendRate).toBe(3000);
  });

  it("returns 0 dailySpendRate when daysElapsed is 0", () => {
    const summary = buildPeriodSummary({ totalSpent: 0, daysElapsed: 0 });

    expect(summary.dailySpendRate).toBe(0);
  });
});

describe("buildDefaults", () => {
  it("returns valid DefaultSettings with all required fields", () => {
    const defaults = buildDefaults();

    expect(defaults.userId).toBe("user-1");
    expect(defaults.budgetAmount).toBe(300000);
    expect(defaults.essentialsPercent).toBe(50);
    expect(defaults.desiresPercent).toBe(30);
    expect(defaults.savingsPercent).toBe(20);
    expect(defaults.currency).toBe("USD");
    expect(defaults.createdAt).toBe("2026-01-15T00:00:00Z");
    expect(defaults.updatedAt).toBe("2026-01-15T00:00:00Z");
  });

  it("accepts partial overrides", () => {
    const defaults = buildDefaults({ budgetAmount: 500000, currency: "EUR" });

    expect(defaults.budgetAmount).toBe(500000);
    expect(defaults.currency).toBe("EUR");
    expect(defaults.essentialsPercent).toBe(50);
  });
});

describe("buildProRataSchedule", () => {
  it("returns valid ProRataSchedule with all required fields", () => {
    const schedule = buildProRataSchedule();

    expect(schedule.id).toMatch(/^prorata-\d+$/);
    expect(schedule.userId).toBe("user-1");
    expect(schedule.proRataGroup).toBe("group-1");
    expect(schedule.name).toBe("Test Pro-Rata");
    expect(schedule.amount).toBe(10000);
    expect(schedule.currency).toBe("USD");
    expect(schedule.expenseType).toBe("essentials");
    expect(schedule.tagId).toBe("tag-1");
    expect(schedule.targetYear).toBe(2026);
    expect(schedule.targetMonth).toBe(2);
    expect(schedule.installmentIndex).toBe(1);
    expect(schedule.installmentTotal).toBe(3);
    expect(schedule.status).toBe("pending");
    expect(schedule.createdAt).toBe("2026-01-15T00:00:00Z");
    expect(schedule.appliedAt).toBeNull();
  });

  it("produces unique incrementing IDs across calls", () => {
    const schedule1 = buildProRataSchedule();
    const schedule2 = buildProRataSchedule();

    expect(schedule1.id).not.toBe(schedule2.id);
  });

  it("accepts partial overrides", () => {
    const schedule = buildProRataSchedule({
      status: "applied",
      appliedAt: "2026-02-01T00:00:00Z",
    });

    expect(schedule.status).toBe("applied");
    expect(schedule.appliedAt).toBe("2026-02-01T00:00:00Z");
  });
});
