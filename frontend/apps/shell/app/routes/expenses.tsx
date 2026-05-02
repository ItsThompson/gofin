import { lazy, Suspense } from "react";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Lazy-load the ExpenseLogPage from the finance remote package.
 */
const ExpenseLogPage = lazy(() =>
  import("@gofin/finance/src/pages/ExpenseLogPage").then((mod) => ({
    default: mod.ExpenseLogPage,
  })),
);

export default function ExpensesRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <Suspense
      fallback={
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-muted-foreground">Loading expense log...</div>
        </div>
      }
    >
      <ExpenseLogPage user={user} />
    </Suspense>
  );
}
