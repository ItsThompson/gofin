/** Route definitions for the finance remote. Consumed by the shell host. */
export const financeRoutes = {
  dashboard: {
    path: "/dashboard",
    component: "DashboardPage",
  },
  settings: {
    path: "/settings",
    component: "SettingsPage",
  },
  newExpense: {
    path: "/expenses/new",
    component: "NewExpensePage",
  },
  expenseLog: {
    path: "/expenses",
    component: "ExpenseLogPage",
  },
  history: {
    path: "/history",
    component: "HistoryPage",
  },
} as const;
