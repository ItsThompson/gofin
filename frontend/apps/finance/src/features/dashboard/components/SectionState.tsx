import type { ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "@gofin/ui/components/button";
import { Card, CardContent } from "@gofin/ui/components/card";
import { Skeleton } from "@gofin/ui/components/skeleton";
import type { SectionState as SectionStateValue } from "../types";

interface SectionStateProps<T> {
  label: string;
  state: SectionStateValue<T>;
  onRetry: () => void;
  emptyMessage: string;
  emptyContent?: ReactNode;
  children: (data: T) => ReactNode;
}

export function SectionState<T>({
  label,
  state,
  onRetry,
  emptyMessage,
  emptyContent,
  children,
}: SectionStateProps<T>) {
  if (state.status === "idle") return null;

  if (state.status === "loading") {
    return (
      <Card aria-label={`${label} loading`} data-testid={`${label}-skeleton`}>
        <CardContent className="space-y-3 py-8">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    );
  }

  if (state.status === "error") {
    return (
      <Card role="alert">
        <CardContent className="flex flex-col items-center gap-3 py-8 text-center">
          <AlertTriangle className="size-6 text-destructive" />
          <p className="text-sm font-medium">Could not load {label}</p>
          <p className="text-sm text-muted-foreground">{state.error.message}</p>
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            <RefreshCw className="size-4" />
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (state.status === "empty") {
    return (
      emptyContent ?? (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            {emptyMessage}
          </CardContent>
        </Card>
      )
    );
  }

  return <>{children(state.data)}</>;
}
