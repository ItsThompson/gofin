import { Skeleton } from "@gofin/ui/components/skeleton";
import { Card, CardContent, CardHeader } from "@gofin/ui/components/card";

/**
 * Skeleton loading state that mimics the active dashboard layout:
 * header, 4-cell summary bar, 3 category gauges, pacing card,
 * and a recent expenses card.
 */
function DashboardSkeleton() {
  return (
    <div className="space-y-6">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Skeleton className="size-6 rounded" />
          <Skeleton className="h-8 w-36" />
        </div>
        <Skeleton className="h-8 w-32 rounded-md" />
      </div>

      {/* Summary bar: 4 cards */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => (
          <Card key={index}>
            <CardContent className="px-4 py-3">
              <Skeleton className="mb-2 h-4 w-20" />
              <Skeleton className="h-7 w-24" />
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Category gauges: 3 cards */}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Card key={index}>
            <CardContent className="px-4 py-3">
              <div className="flex justify-between">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-8" />
              </div>
              <Skeleton className="mt-2 h-2 w-full rounded-full" />
              <div className="mt-2 flex justify-between">
                <Skeleton className="h-3 w-28" />
                <Skeleton className="h-3 w-16" />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Pacing indicator */}
      <Card>
        <CardHeader className="pb-2">
          <Skeleton className="h-5 w-32" />
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <div key={index}>
                <Skeleton className="mb-1 h-3 w-20" />
                <Skeleton className="h-6 w-28" />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Recent expenses */}
      <Card>
        <CardHeader>
          <div className="flex justify-between">
            <Skeleton className="h-6 w-36" />
            <Skeleton className="h-4 w-16" />
          </div>
        </CardHeader>
        <CardContent>
          <div className="divide-y">
            {Array.from({ length: 5 }).map((_, index) => (
              <div
                key={index}
                className="flex items-center justify-between py-3"
              >
                <div className="flex flex-col gap-1">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-3 w-20" />
                </div>
                <Skeleton className="h-4 w-16" />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

/**
 * Skeleton for the expense log page: header, controls row, table rows.
 */
function ExpenseLogSkeleton() {
  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Skeleton className="size-6 rounded" />
        <Skeleton className="h-8 w-32" />
      </div>

      {/* Controls row */}
      <div className="flex flex-wrap items-center gap-3">
        <Skeleton className="h-8 w-48 rounded-md" />
        <Skeleton className="h-8 w-24 rounded-md" />
        <Skeleton className="ml-auto h-4 w-20" />
      </div>

      {/* Desktop table */}
      <div className="hidden md:block">
        <Card>
          <CardContent className="p-0">
            {/* Header row */}
            <div className="flex gap-4 border-b px-4 py-3">
              {["w-24", "w-36", "w-16", "w-20", "w-20", "w-16"].map(
                (width, index) => (
                  <Skeleton key={index} className={`h-4 ${width}`} />
                ),
              )}
            </div>
            {/* Data rows */}
            {Array.from({ length: 8 }).map((_, index) => (
              <div
                key={index}
                className="flex gap-4 border-b px-4 py-3 last:border-0"
              >
                {["w-24", "w-36", "w-16", "w-20", "w-20", "w-16"].map(
                  (width, cellIndex) => (
                    <Skeleton
                      key={cellIndex}
                      className={`h-4 ${width}`}
                    />
                  ),
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      </div>

      {/* Mobile list */}
      <div className="md:hidden">
        <Card>
          <CardContent className="p-0">
            <div className="divide-y">
              {Array.from({ length: 8 }).map((_, index) => (
                <div
                  key={index}
                  className="flex items-center justify-between px-4 py-3"
                >
                  <div className="flex flex-col gap-1">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-3 w-20" />
                  </div>
                  <Skeleton className="h-4 w-16" />
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-4 w-20" />
        <div className="flex gap-1">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="size-8 rounded-md" />
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * Skeleton for the settings page: sidebar tabs + content area.
 */
function SettingsSkeleton() {
  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Skeleton className="size-6 rounded" />
        <Skeleton className="h-8 w-24" />
      </div>

      {/* Desktop: tabs + content */}
      <div className="hidden md:flex gap-6">
        <div className="flex flex-col gap-1 min-w-[180px]">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-9 w-full rounded-lg" />
          ))}
        </div>
        <Card className="flex-1">
          <CardHeader>
            <Skeleton className="h-6 w-32" />
          </CardHeader>
          <CardContent className="space-y-4">
            {Array.from({ length: 4 }).map((_, index) => (
              <div key={index} className="space-y-1.5">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-8 w-full rounded-md" />
              </div>
            ))}
            <Skeleton className="h-9 w-28 rounded-md" />
          </CardContent>
        </Card>
      </div>

      {/* Mobile: accordion cards */}
      <div className="flex flex-col gap-2 md:hidden">
        {Array.from({ length: 4 }).map((_, index) => (
          <Card key={index}>
            <div className="flex items-center justify-between px-4 py-3">
              <div className="flex items-center gap-2">
                <Skeleton className="size-4 rounded" />
                <Skeleton className="h-4 w-24" />
              </div>
              <Skeleton className="size-3" />
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}

export { DashboardSkeleton, ExpenseLogSkeleton, SettingsSkeleton };
