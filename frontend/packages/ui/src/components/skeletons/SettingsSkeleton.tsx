import { Skeleton } from "@gofin/ui/components/skeleton";
import { Card, CardContent, CardHeader } from "@gofin/ui/components/card";

/**
 * Skeleton for the settings page: sidebar tabs + content area.
 */
export function SettingsSkeleton() {
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
