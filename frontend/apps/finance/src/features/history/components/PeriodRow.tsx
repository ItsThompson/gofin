import { formatCurrency } from "@gofin/core";
import { Card, CardContent } from "@gofin/ui/components/card";
import { ArrowRight } from "lucide-react";
import type { HistoricalPeriodRow } from "../types";
import { PeriodSpendFigures } from "./PeriodSpendFigures";

interface PeriodRowProps {
  row: HistoricalPeriodRow;
  onSelect: () => void;
}

export function PeriodRow({ row, onSelect }: PeriodRowProps) {
  const reportingCurrencyCode = row.period.reportingCurrencyCode;
  const monthName = new Date(
    row.period.year,
    row.period.month - 1,
  ).toLocaleString("en-US", { month: "long", year: "numeric" });

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
            Budget: {formatCurrency(row.period.budgetAmount, reportingCurrencyCode)}{" "}
            · E/D/S: {row.period.essentialsPercent}/{row.period.desiresPercent}/
            {row.period.savingsPercent}
          </span>
        </div>
        <div className="flex items-center gap-4">
          <div className="text-right">
            {row.status === "loaded" ? (
              <PeriodSpendFigures
                totalSpent={row.totalSpent}
                surplus={row.surplus}
                currency={reportingCurrencyCode}
                delta={row.deltaFromPrevious}
              />
            ) : (
              <>
                <p className="text-sm text-muted-foreground">
                  Spent: unavailable
                </p>
                <p className="text-sm text-muted-foreground">
                  Could not load this month
                </p>
              </>
            )}
          </div>
          <ArrowRight className="size-4 text-muted-foreground" />
        </div>
      </CardContent>
    </Card>
  );
}