import { formatCurrency } from "@gofin/core";

interface PeriodSpendFiguresProps {
  /** Amount spent in the period, in minor currency units. */
  totalSpent: number;
  /** budgetAmount minus totalSpent. Negative means a deficit. */
  surplus: number;
  currency: string;
}

/** Spend and surplus figures for a period whose summary loaded. */
export function PeriodSpendFigures({
  totalSpent,
  surplus,
  currency,
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
    </>
  );
}
