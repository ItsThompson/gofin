import type { Expense } from "@gofin/core";
import { uuid, daysAgoISO, currentYear, currentMonth } from "./foundation";
import { adminUser } from "./users";
import { tagIds } from "./tags";

export const mockExpenses: Expense[] = [
  {
    id: uuid(), userId: adminUser.id, name: "Rent", amount: 120000,
    transactionCurrency: "USD", expenseType: "essentials", tagId: tagIds.bills,
    transactionAmount: 120000, reportingAmount: 120000, reportingCurrency: "USD",
    expenseDate: daysAgoISO(0), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Grocery run", amount: 8540,
    transactionCurrency: "USD", expenseType: "essentials", tagId: tagIds.food,
    transactionAmount: 8540, reportingAmount: 8540, reportingCurrency: "USD",
    expenseDate: daysAgoISO(1), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Concert tickets", amount: 15000,
    transactionCurrency: "USD", expenseType: "desires", tagId: tagIds.recreation,
    transactionAmount: 15000, reportingAmount: 15000, reportingCurrency: "USD",
    expenseDate: daysAgoISO(2), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Bus pass", amount: 7500,
    transactionCurrency: "USD", expenseType: "essentials", tagId: tagIds.transport,
    transactionAmount: 7500, reportingAmount: 7500, reportingCurrency: "USD",
    expenseDate: daysAgoISO(3), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Online course", amount: 4999,
    transactionCurrency: "USD", expenseType: "savings", tagId: tagIds.selfInvestment,
    transactionAmount: 4999, reportingAmount: 4999, reportingCurrency: "USD",
    expenseDate: daysAgoISO(4), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Dinner with friends", amount: 4200,
    transactionCurrency: "USD", expenseType: "desires", tagId: tagIds.social,
    transactionAmount: 4200, reportingAmount: 4200, reportingCurrency: "USD",
    expenseDate: daysAgoISO(5), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Coffee beans", amount: 1899,
    transactionCurrency: "USD", expenseType: "desires", tagId: tagIds.coffee,
    transactionAmount: 1899, reportingAmount: 1899, reportingCurrency: "USD",
    expenseDate: daysAgoISO(6), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
];
