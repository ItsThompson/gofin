import { lazy } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { RemoteBoundary } from "@/components/remote-boundary";
import { ExpenseLogSkeleton } from "@gofin/ui/components/skeletons";
import { accessHandle } from "@/lib/route-access";

/**
 * Lazy-load the ExpenseLogFeature from the finance package.
 */
const ExpenseLogFeature = lazy(() =>
  import("@gofin/finance/src/features/expense-log").then((mod) => ({
    default: mod.ExpenseLogFeature,
  })),
);

export const handle = accessHandle("personal");

export default function ExpensesRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <RemoteBoundary
      sectionName="Expense Log"
      loadingFallback={<ExpenseLogSkeleton />}
    >
      <ExpenseLogFeature user={user} />
    </RemoteBoundary>
  );
}
