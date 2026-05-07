import { formatCurrency } from "@gofin/core";
import type { PeriodSummary } from "../../../../types";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  Activity,
  TrendingUp,
  AlertTriangle,
  Target,
} from "lucide-react";

interface PacingIndicatorProps {
  summary: PeriodSummary;
  currency: string;
}

export function PacingIndicator({ summary, currency }: PacingIndicatorProps) {
  const isOverBudget = summary.totalSpent > summary.totalBudget;
  const overAmount = isOverBudget ? summary.totalSpent - summary.totalBudget : 0;

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <Activity className="size-4 text-muted-foreground" />
          <CardTitle className="text-base">Spending Pace</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div>
            <p className="text-xs text-muted-foreground">Daily Average</p>
            <p className="text-lg font-semibold">
              {formatCurrency(summary.dailySpendRate, currency)}/day
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Required Rate</p>
            <p className="text-lg font-semibold">
              {summary.budgetPace > 0
                ? `${formatCurrency(summary.budgetPace, currency)}/day`
                : "N/A"}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Status</p>
            {isOverBudget ? (
              <div className="flex items-center gap-1">
                <TrendingUp className="size-4 text-red-600 dark:text-red-400" />
                <p className="text-lg font-semibold text-red-600 dark:text-red-400">
                  Over by {formatCurrency(overAmount, currency)}
                </p>
              </div>
            ) : summary.isOnTrack ? (
              <div className="flex items-center gap-1">
                <Target className="size-4 text-green-600 dark:text-green-400" />
                <p className="text-lg font-semibold text-green-600 dark:text-green-400">
                  On Track
                </p>
              </div>
            ) : (
              <div className="flex items-center gap-1">
                <AlertTriangle className="size-4 text-yellow-600 dark:text-yellow-400" />
                <p className="text-lg font-semibold text-yellow-600 dark:text-yellow-400">
                  Over Pace
                </p>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
