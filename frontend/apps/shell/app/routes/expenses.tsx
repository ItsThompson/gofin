import { lazy } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { RemoteBoundary } from "@/components/remote-boundary";
import { ExpenseLogSkeleton } from "@gofin/ui/components/skeletons";

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
    <RemoteBoundary
      sectionName="Expense Log"
      loadingFallback={<ExpenseLogSkeleton />}
    >
      <ExpenseLogPage user={user} />
    </RemoteBoundary>
  );
}
