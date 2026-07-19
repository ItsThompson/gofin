import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import type { HealthScore } from "../../../../types";
import { ScoreRing } from "./ScoreRing";
import { SubScoreBars } from "./SubScoreBars";
import { InsightPanel } from "./InsightPanel";
import { HealthScoreConfigurePrompt } from "./HealthScoreConfigurePrompt";

interface HealthScoreCardProps {
  /** The health score, or null when the fetch failed or returned 404. */
  score: HealthScore | null;
}

/**
 * Dashboard card for the monthly financial health score. Routes between the
 * no-score, configure-budget, and scored states. Money strings are formatted
 * server-side, so the card needs no currency.
 */
export function HealthScoreCard({ score }: HealthScoreCardProps) {
  if (!score) {
    return null;
  }

  if (score.configureBudget) {
    return <HealthScoreConfigurePrompt />;
  }

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="text-base">Financial Health</CardTitle>
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
        {/* Phase 2: monthly-score trend sparkline goes here. */}
      </CardContent>
    </Card>
  );
}
