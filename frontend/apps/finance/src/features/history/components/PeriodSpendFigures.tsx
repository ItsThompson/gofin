import { formatCurrency } from "@gofin/core";
import type { PeriodDelta } from "../types";

interface PeriodSpendFiguresProps {
  /** Amount spent in the period, in minor currency units. */
  totalSpent: number;
  /** budgetAmount minus totalSpent. Negative means a deficit. */
  surplus: number;
  currency: string;
  /** Delta from the previous period, when both rows loaded. */
  delta?: PeriodDelta;
}

/** Spend and surplus figures for a period whose summary loaded. */
export function PeriodSpendFigures({
  totalSpent,
  surplus,
  currency,
  delta,
}: PeriodSpendFiguresProps) {
  const isSurplus = surplus >= 0;

  return (
    <>
      <p className="text-sm">Spent: {formatCurrency(totalSpent, currency)}</p>
      <p
        className={`text-sm font-semibold ${
          isSurplus
            ? "text-green-600 dark:text-green-400"
            : "text-red-600 dark:text-red-400"
        }`}
      >
        {isSurplus ? "Surplus" : "Deficit"}:{" "}
        {formatCurrency(Math.abs(surplus), currency)}
      </p>
      {delta && (
        <p className="text-xs text-muted-foreground" data-testid="period-delta">
          {delta.comparable
            ? `Δ ${delta.amount >= 0 ? "+" : ""}${formatCurrency(Math.abs(delta.amount), currency)} from last`
            : "Δ not comparable (different currency)"}
        </p>
      )}
    </>
  );
}