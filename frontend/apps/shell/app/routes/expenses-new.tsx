import { lazy, Suspense } from "react";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Lazy-load the NewExpensePage from the finance remote package.
 */
const NewExpensePage = lazy(() =>
  import("@gofin/finance/src/pages/NewExpensePage").then((mod) => ({
    default: mod.NewExpensePage,
  })),
);

export default function NewExpenseRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <Suspense
      fallback={
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-muted-foreground">Loading expense form...</div>
        </div>
      }
    >
      <NewExpensePage user={user} />
    </Suspense>
  );
}
