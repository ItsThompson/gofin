import { formatCurrency } from "@gofin/core";
import type { HistoricalComparison } from "@gofin/core";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  History,
  TrendingUp,
  TrendingDown,
} from "lucide-react";

interface HistoricalComparisonWidgetProps {
  comparison: HistoricalComparison;
  currency: string;
}

export function HistoricalComparisonWidget({
  comparison,
  currency,
}: HistoricalComparisonWidgetProps) {
  const hasPrevious = comparison.previousSpent > 0 || comparison.currentSpent > 0;
  const isOnlyOnePeriod = comparison.previousSpent === 0 && comparison.changePercent === 0;

  return (
    <Card data-testid="historical-comparison" className="h-full">
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <History className="size-4 text-muted-foreground" />
          <CardTitle className="text-base">Historical Comparison</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        {isOnlyOnePeriod ? (
          <p className="text-sm text-muted-foreground">
            Not enough data for comparison
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div>
              <p className="text-xs text-muted-foreground">Current Period</p>
              <p className="text-lg font-semibold">
                {formatCurrency(comparison.currentSpent, currency)}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Previous Period</p>
              <p className="text-lg font-semibold">
                {formatCurrency(comparison.previousSpent, currency)}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">
                {comparison.rollingAverage != null ? "Rolling Average (3mo)" : "Change"}
              </p>
              {comparison.rollingAverage != null ? (
                <p className="text-lg font-semibold">
                  {formatCurrency(comparison.rollingAverage, currency)}
                </p>
              ) : null}
              {hasPrevious && (
                <div className="flex items-center gap-1 mt-1">
                  {comparison.changePercent > 0 ? (
                    <TrendingUp className="size-4 text-red-600 dark:text-red-400" />
                  ) : comparison.changePercent < 0 ? (
                    <TrendingDown className="size-4 text-green-600 dark:text-green-400" />
                  ) : null}
                  <span
                    className={`text-sm font-medium ${
                      comparison.changePercent > 0
                        ? "text-red-600 dark:text-red-400"
                        : comparison.changePercent < 0
                          ? "text-green-600 dark:text-green-400"
                          : "text-muted-foreground"
                    }`}
                  >
                    {comparison.changePercent > 0 ? "+" : ""}
                    {comparison.changePercent.toFixed(1)}% from last period
                  </span>
                </div>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
