import type { ProRataSchedule } from "@gofin/core";
import { uuid, currentYear, currentMonth } from "./foundation";
import { adminUser } from "./users";
import { tagIds } from "./tags";

export const mockUpcomingProRata: ProRataSchedule[] = [
  {
    id: uuid(),
    userId: adminUser.id,
    proRataGroup: uuid(),
    name: "New laptop",
    amount: 50000, // $500 installment
    transactionCurrencyCode: "USD",
    expenseType: "essentials",
    tagId: tagIds.selfInvestment,
    targetYear: currentMonth === 12 ? currentYear + 1 : currentYear,
    targetMonth: currentMonth === 12 ? 1 : currentMonth + 1,
    installmentIndex: 2,
    installmentTotal: 3,
    status: "pending",
    createdAt: "2026-04-01T00:00:00Z",
    appliedAt: null,
  },
];
