import { cn } from "@gofin/ui/lib/utils";

interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  className?: string;
}

/**
 * Base skeleton pulse element. Renders a rounded rectangle with a
 * shimmer animation. Compose multiple Skeleton elements to build
 * layout-specific loading states.
 */
function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div
      data-slot="skeleton"
      className={cn("animate-pulse rounded-md bg-muted", className)}
      {...props}
    />
  );
}

export { Skeleton };
