import { formatCurrency } from "@gofin/core";
import type { PeriodSummary } from "@gofin/core";
import {
  Card,
  CardContent,
} from "@gofin/ui/components/card";

interface CategoryGaugesProps {
  summary: PeriodSummary;
  currency: string;
}

export function CategoryGauges({ summary, currency }: CategoryGaugesProps) {
  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
      <CategoryGauge
        label="Essentials"
        category={summary.essentials}
        currency={currency}
      />
      <CategoryGauge
        label="Desires"
        category={summary.desires}
        currency={currency}
      />
      <CategoryGauge
        label="Savings"
        category={summary.savings}
        currency={currency}
      />
    </div>
  );
}

function CategoryGauge({
  label,
  category,
  currency,
}: {
  label: string;
  category: PeriodSummary["essentials"];
  currency: string;
}) {
  const isOverBudget = category.percentUsed >= 100;
  const progressPercent = Math.min(category.percentUsed, 100);

  return (
    <Card data-testid={`gauge-${label.toLowerCase()}`}>
      <CardContent className="px-4 py-3">
        <div className="flex items-center justify-between text-sm">
          <span className="font-medium">{label}</span>
          <span
            className={`text-xs font-semibold ${isOverBudget ? "text-red-600 dark:text-red-400" : "text-muted-foreground"}`}
          >
            {category.percentUsed.toFixed(0)}%
          </span>
        </div>
        <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-muted">
          <div
            className={`h-full rounded-full transition-all ${isOverBudget ? "bg-red-500" : "bg-primary"}`}
            style={{ width: `${progressPercent}%` }}
            role="progressbar"
            aria-valuenow={category.percentUsed}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`${label} budget usage`}
          />
        </div>
        <div className="mt-2 flex justify-between text-xs text-muted-foreground">
          <span>
            {formatCurrency(category.spent, currency)} of{" "}
            {formatCurrency(category.allocated, currency)}
          </span>
          <span>
            {isOverBudget ? (
              <span className="text-red-600 dark:text-red-400">
                Over by {formatCurrency(Math.abs(category.remaining), currency)}
              </span>
            ) : (
              <span>
                {formatCurrency(category.remaining, currency)} left
              </span>
            )}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
