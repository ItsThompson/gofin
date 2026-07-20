import type { Tag } from "@gofin/core";
import { uuid } from "./foundation";

export const tagIds = {
  bills: uuid(),
  food: uuid(),
  household: uuid(),
  investment: uuid(),
  personalCare: uuid(),
  recreation: uuid(),
  selfInvestment: uuid(),
  social: uuid(),
  transport: uuid(),
  travel: uuid(),
  coffee: uuid(),
};

export const mockTags: Tag[] = [
  { id: tagIds.bills, name: "Bills", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.coffee, name: "Coffee", isDefault: false, createdAt: "2026-04-15T00:00:00Z", updatedAt: "2026-04-15T00:00:00Z" },
  { id: tagIds.food, name: "Food", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.household, name: "Household", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.investment, name: "Investment", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.personalCare, name: "Personal Care", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.recreation, name: "Recreation/Entertainment", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.selfInvestment, name: "Self Investment", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.social, name: "Social", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.transport, name: "Transport", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
  { id: tagIds.travel, name: "Travel", isDefault: true, createdAt: "2026-04-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
];
