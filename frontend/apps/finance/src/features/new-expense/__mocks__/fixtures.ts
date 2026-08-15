import { buildUser, buildTag } from "@gofin/test-utils";

import type { ExpenseSuggestionsResponse } from "../../expense-autocomplete";
import type { BudgetPeriod, Tag } from "@gofin/core";

export const mockUser = buildUser({
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
});

export const mockPeriod: BudgetPeriod = {
  id: "period-1",
  userId: "user-1",
  year: 2026,
  month: 5,
  budgetAmount: 500000,
  reportingCurrency: "USD",
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
};

export const mockTags: Tag[] = [
  buildTag({ id: "tag-bills", name: "Bills", isDefault: true }),
  buildTag({ id: "tag-food", name: "Food", isDefault: true }),
];

export const mockSuggestions: ExpenseSuggestionsResponse = {
  data: [
    {
      name: "Coffee Shop",
      transactionAmount: 450,
      transactionCurrency: "USD",
      amount: 450,
      currency: "USD",
      expenseType: "desires",
      tagId: "tag-food",
      frequency: 4,
      lastUsedAt: "2026-05-25T00:00:00Z",
      recencyBucket: "last_7_days",
      frecencyScore: 42,
    },
    {
      name: "Coffee Beans",
      transactionAmount: 1200,
      transactionCurrency: "USD",
      amount: 1200,
      currency: "USD",
      expenseType: "essentials",
      tagId: "tag-food",
      frequency: 2,
      lastUsedAt: "2026-05-20T00:00:00Z",
      recencyBucket: "last_30_days",
      frecencyScore: 31,
    },
  ],
  total: 2,
  page: 1,
  pageSize: 50,
  hasMore: false,
};

export const emptySuggestions: ExpenseSuggestionsResponse = {
  data: [],
  total: 0,
  page: 1,
  pageSize: 50,
  hasMore: false,
};
