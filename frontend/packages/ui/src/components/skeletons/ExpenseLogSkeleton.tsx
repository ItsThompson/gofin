import { Skeleton } from "@gofin/ui/components/skeleton";
import { Card, CardContent } from "@gofin/ui/components/card";

/**
 * Skeleton for the expense log page: header, controls row, table rows.
 */
export function ExpenseLogSkeleton() {
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
