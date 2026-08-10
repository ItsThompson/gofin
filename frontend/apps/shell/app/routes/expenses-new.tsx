import { lazy } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { RemoteBoundary } from "@/components/remote-boundary";
import { Skeleton } from "@gofin/ui/components/skeleton";
import { Card, CardContent, CardHeader } from "@gofin/ui/components/card";
import { accessHandle } from "@/lib/route-access";

/**
 * Lazy-load the NewExpenseFeature from the finance package.
 */
const NewExpenseFeature = lazy(() =>
  import("@gofin/finance/src/features/new-expense").then((mod) => ({
    default: mod.NewExpenseFeature,
  })),
);

function ExpenseFormSkeleton() {
  return (
    <div className="flex items-start justify-center pt-4 md:pt-8">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <div className="flex items-center gap-3">
            <Skeleton className="size-6 rounded" />
            <Skeleton className="h-8 w-32" />
          </div>
          <Skeleton className="mt-1 h-4 w-48" />
        </CardHeader>
        <CardContent className="space-y-4">
          {Array.from({ length: 5 }).map((_, index) => (
            <div key={index} className="space-y-1.5">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-8 w-full rounded-md" />
            </div>
          ))}
          <Skeleton className="h-10 w-full rounded-md" />
        </CardContent>
      </Card>
    </div>
  );
}

export const handle = accessHandle("personal");

export default function NewExpenseRoute() {
  const { user } = useAuthStore();

  if (!user) return null;

  return (
    <RemoteBoundary
      sectionName="Expense Form"
      loadingFallback={<ExpenseFormSkeleton />}
    >
      <NewExpenseFeature user={user} />
    </RemoteBoundary>
  );
}
