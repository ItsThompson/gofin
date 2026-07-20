import { Lightbulb } from "lucide-react";
import type { HealthInsight } from "@gofin/core";

interface InsightPanelProps {
  insight: HealthInsight;
}

/** The plain-English read: summary plus the one-line nudge. */
export function InsightPanel({ insight }: InsightPanelProps) {
  return (
    <div className="flex gap-3 rounded-lg bg-muted/40 p-3">
      <Lightbulb
        className="mt-0.5 size-4 shrink-0 text-amber-500"
        aria-hidden="true"
      />
      <div className="space-y-1">
        <p className="text-sm font-medium">{insight.summary}</p>
        <p className="text-xs text-muted-foreground">{insight.nudge}</p>
      </div>
    </div>
  );
}
