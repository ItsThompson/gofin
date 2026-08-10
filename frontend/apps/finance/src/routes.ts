/** Route definitions for the finance package. Consumed by the shell. */
export const financeRoutes = {
  dashboard: {
    path: "/dashboard",
    component: "DashboardFeature",
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
