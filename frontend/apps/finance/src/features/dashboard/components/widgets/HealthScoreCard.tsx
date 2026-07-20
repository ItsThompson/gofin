import { useState } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import { Info } from "lucide-react";
import type {
  HealthScore,
  HealthScoreConfigureBudget,
  HealthScoreTrendPoint,
} from "@gofin/core";
import { ScoreRing } from "./ScoreRing";
import { SubScoreBars } from "./SubScoreBars";
import { InsightPanel } from "./InsightPanel";
import { HealthScoreConfigurePrompt } from "./HealthScoreConfigurePrompt";
import { HealthScoreSparkline } from "./HealthScoreSparkline";
import { HealthScoreInfoModal } from "./HealthScoreInfoModal";

interface HealthScoreCardProps {
  /** The score, the configure-budget prompt, or null when the fetch failed. */
  score: HealthScore | HealthScoreConfigureBudget | null;
  /** Recent monthly scores for the sparkline; null when unavailable. */
  trend?: HealthScoreTrendPoint[] | null;
}

/**
 * Dashboard card for the monthly financial health score. Routes between the
 * no-score, configure-budget, and scored states. Money strings are formatted
 * server-side, so the card needs no currency.
 */
export function HealthScoreCard({ score, trend }: HealthScoreCardProps) {
  const [infoOpen, setInfoOpen] = useState(false);

  if (!score) {
    return null;
  }

  if ("configureBudget" in score) {
    return <HealthScoreConfigurePrompt />;
  }

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5">
            <CardTitle className="text-base">Financial Health</CardTitle>
            <button
              type="button"
              onClick={() => setInfoOpen(true)}
              aria-label="About the Financial Health Score"
              className="rounded-full p-0.5 text-muted-foreground transition-colors hover:text-foreground"
            >
              <Info className="size-4" aria-hidden="true" />
            </button>
          </div>
          {score.provisional && (
            <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
              Month to date
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <ScoreRing total={score.total} band={score.band} />
        <SubScoreBars
          components={score.components}
          driverKey={score.insight.driver}
        />
        <InsightPanel insight={score.insight} />
        {trend && <HealthScoreSparkline points={trend} />}
      </CardContent>
      <HealthScoreInfoModal open={infoOpen} onClose={() => setInfoOpen(false)} />
    </Card>
  );
}
