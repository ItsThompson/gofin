import type { BudgetPeriod, DefaultSettings } from "@gofin/core";
import { currentYear, currentMonth } from "./foundation";
import { adminUser } from "./users";

export const mockPeriod: BudgetPeriod = {
  id: "bp-001",
  userId: adminUser.id,
  year: currentYear,
  month: currentMonth,
  budgetAmount: 300000, // $3,000
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  createdAt: `${currentYear}-${String(currentMonth).padStart(2, "0")}-01T00:00:00Z`,
  updatedAt: `${currentYear}-${String(currentMonth).padStart(2, "0")}-01T00:00:00Z`,
};

export const mockDefaults: DefaultSettings = {
  userId: adminUser.id,
  budgetAmount: 300000,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  currency: "USD",
  createdAt: "2026-04-01T00:00:00Z",
  updatedAt: "2026-04-01T00:00:00Z",
};
