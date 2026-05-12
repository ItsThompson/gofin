import { formatCurrency } from "@gofin/core";
import { Card, CardContent } from "@gofin/ui/components/card";
import { ArrowRight } from "lucide-react";
import type { HistoricalPeriodRow } from "../hooks/useHistoryData";

interface PeriodRowProps {
  row: HistoricalPeriodRow;
  currency: string;
  onSelect: () => void;
}

export function PeriodRow({ row, currency, onSelect }: PeriodRowProps) {
  const monthName = new Date(
    row.period.year,
    row.period.month - 1,
  ).toLocaleString("en-US", { month: "long", year: "numeric" });
  const isSurplus = row.surplus >= 0;

  return (
    <Card
      className="cursor-pointer transition-colors hover:bg-muted/50"
      onClick={onSelect}
      data-testid={`period-row-${row.period.year}-${row.period.month}`}
    >
      <CardContent className="flex items-center justify-between px-4 py-3">
        <div className="flex flex-col gap-0.5">
          <span className="font-medium">{monthName}</span>
          <span className="text-xs text-muted-foreground">
            Budget: {formatCurrency(row.period.budgetAmount, currency)}{" "}
            · E/D/S: {row.period.essentialsPercent}/{row.period.desiresPercent}/
            {row.period.savingsPercent}
          </span>
        </div>
        <div className="flex items-center gap-4">
          <div className="text-right">
            <p className="text-sm">
              Spent: {formatCurrency(row.totalSpent, currency)}
            </p>
            <p
              className={`text-sm font-semibold ${
                isSurplus
                  ? "text-green-600 dark:text-green-400"
                  : "text-red-600 dark:text-red-400"
              }`}
            >
              {isSurplus ? "Surplus" : "Deficit"}:{" "}
              {formatCurrency(Math.abs(row.surplus), currency)}
            </p>
          </div>
          <ArrowRight className="size-4 text-muted-foreground" />
        </div>
      </CardContent>
    </Card>
  );
}
