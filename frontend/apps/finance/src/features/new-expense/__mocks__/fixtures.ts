import { buildUser, buildTag } from "@gofin/test-utils";

import type { ExpenseSuggestionsResponse } from "../../expense-autocomplete";
import type { Tag } from "../../../types";

export const mockUser = buildUser({
  id: "user-1",
  username: "alice",
  email: "alice@example.com",
});

export const mockTags: Tag[] = [
  buildTag({ id: "tag-bills", name: "Bills", isDefault: true }),
  buildTag({ id: "tag-food", name: "Food", isDefault: true }),
];

export const mockSuggestions: ExpenseSuggestionsResponse = {
  data: [
    {
      name: "Coffee Shop",
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
