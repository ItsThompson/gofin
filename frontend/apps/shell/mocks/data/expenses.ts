import type { Expense } from "@gofin/core";
import { uuid, daysAgoISO, currentYear, currentMonth } from "./foundation";
import { adminUser } from "./users";
import { tagIds } from "./tags";

export const mockExpenses: Expense[] = [
  {
    id: uuid(), userId: adminUser.id, name: "Rent", amount: 120000,
    currency: "USD", transactionCurrency: "USD", expenseType: "essentials", tagId: tagIds.bills,
    expenseDate: daysAgoISO(0), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Grocery run", amount: 8540,
    currency: "USD", transactionCurrency: "USD", expenseType: "essentials", tagId: tagIds.food,
    expenseDate: daysAgoISO(1), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Concert tickets", amount: 15000,
    currency: "USD", transactionCurrency: "USD", expenseType: "desires", tagId: tagIds.recreation,
    expenseDate: daysAgoISO(2), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Bus pass", amount: 7500,
    currency: "USD", transactionCurrency: "USD", expenseType: "essentials", tagId: tagIds.transport,
    expenseDate: daysAgoISO(3), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Online course", amount: 4999,
    currency: "USD", transactionCurrency: "USD", expenseType: "savings", tagId: tagIds.selfInvestment,
    expenseDate: daysAgoISO(4), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Dinner with friends", amount: 4200,
    currency: "USD", transactionCurrency: "USD", expenseType: "desires", tagId: tagIds.social,
    expenseDate: daysAgoISO(5), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
  {
    id: uuid(), userId: adminUser.id, name: "Coffee beans", amount: 1899,
    currency: "USD", transactionCurrency: "USD", expenseType: "desires", tagId: tagIds.coffee,
    expenseDate: daysAgoISO(6), periodYear: currentYear, periodMonth: currentMonth,
    status: "active", isProRata: false, createdAt: new Date().toISOString(),
  },
];
