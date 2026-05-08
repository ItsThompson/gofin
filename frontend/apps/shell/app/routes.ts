import {
  type RouteConfig,
  index,
  layout,
  route,
} from "@react-router/dev/routes";

export default [
  // Public routes (no auth required, redirect if already logged in)
  route("login", "routes/login.tsx"),
  route("register", "routes/register.tsx"),

  // Authenticated routes wrapped in auth layout (navbar, auth guard)
  layout("routes/auth-layout.tsx", [
    index("routes/home.tsx"),
    route("dashboard", "routes/dashboard.tsx"),
    route("onboarding", "routes/onboarding.tsx"),
    route("expenses", "routes/expenses.tsx"),
    route("expenses/new", "routes/expenses-new.tsx"),
    route("history", "routes/history.tsx"),
    route("settings", "routes/settings.tsx"),
    route("admin", "routes/admin.tsx"),
    route("admin/users", "routes/admin-users.tsx"),
  ]),
] satisfies RouteConfig;
