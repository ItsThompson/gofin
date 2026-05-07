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
    component: "NewExpenseFeature",
  },
  expenseLog: {
    path: "/expenses",
    component: "ExpenseLogFeature",
  },
  history: {
    path: "/history",
    component: "HistoryFeature",
  },
} as const;
