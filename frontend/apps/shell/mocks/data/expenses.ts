import type { Expense } from "@gofin/core";
import { uuid, daysAgoISO, currentYear, currentMonth } from "./foundation";
import { adminUser } from "./users";
import { tagIds } from "./tags";

export const mockExpenses: Expense[] = [
  {
    id: uuid(), userId: adminUser.id, name: "Rent",
    transactionCurrencyCode: "USD", expenseType: "essentials", tagId: tagIds.bills,
    originalTransactionAmountInMinorUnits: 120000, reportingAmountInMinorUnits: 120000, reportingCurrencyCode: "USD",
    expenseDateIso: daysAgoISO(0), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Grocery run",
    transactionCurrencyCode: "USD", expenseType: "essentials", tagId: tagIds.food,
    originalTransactionAmountInMinorUnits: 8540, reportingAmountInMinorUnits: 8540, reportingCurrencyCode: "USD",
    expenseDateIso: daysAgoISO(1), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Concert tickets",
    transactionCurrencyCode: "USD", expenseType: "desires", tagId: tagIds.recreation,
    originalTransactionAmountInMinorUnits: 15000, reportingAmountInMinorUnits: 15000, reportingCurrencyCode: "USD",
    expenseDateIso: daysAgoISO(2), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Bus pass",
    transactionCurrencyCode: "USD", expenseType: "essentials", tagId: tagIds.transport,
    originalTransactionAmountInMinorUnits: 7500, reportingAmountInMinorUnits: 7500, reportingCurrencyCode: "USD",
    expenseDateIso: daysAgoISO(3), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Online course",
    transactionCurrencyCode: "USD", expenseType: "savings", tagId: tagIds.selfInvestment,
    originalTransactionAmountInMinorUnits: 4999, reportingAmountInMinorUnits: 4999, reportingCurrencyCode: "USD",
    expenseDateIso: daysAgoISO(4), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Dinner with friends",
    transactionCurrencyCode: "USD", expenseType: "desires", tagId: tagIds.social,
    originalTransactionAmountInMinorUnits: 4200, reportingAmountInMinorUnits: 4200, reportingCurrencyCode: "USD",
    expenseDateIso: daysAgoISO(5), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Coffee beans",
    transactionCurrencyCode: "USD", expenseType: "desires", tagId: tagIds.coffee,
    originalTransactionAmountInMinorUnits: 1899, reportingAmountInMinorUnits: 1899, reportingCurrencyCode: "USD",
    expenseDateIso: daysAgoISO(6), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
];
