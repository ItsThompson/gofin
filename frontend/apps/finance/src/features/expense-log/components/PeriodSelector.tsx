import type { BudgetPeriod } from "@gofin/core";

interface PeriodSelectorProps {
  periods: BudgetPeriod[];
  selectedYear: number;
  selectedMonth: number;
  onChange: (value: string) => void;
}

export function PeriodSelector({
  periods,
  selectedYear,
  selectedMonth,
  onChange,
}: PeriodSelectorProps) {
  const periodValue = `${selectedYear}-${selectedMonth}`;

  return (
    <div className="flex items-center gap-2">
      <label htmlFor="period-select" className="text-sm font-medium">
        Period:
      </label>
      <select
        id="period-select"
        value={periodValue}
        onChange={(event) => onChange(event.target.value)}
        className="h-8 rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        {periods.length > 0 ? (
          periods.map((period) => {
            const optionValue = `${period.year}-${period.month}`;
            const label = new Date(
              period.year,
              period.month - 1,
            ).toLocaleString("en-US", {
              month: "long",
              year: "numeric",
            });
            return (
              <option key={optionValue} value={optionValue}>
                {label}
              </option>
            );
          })
        ) : (
          <option value={periodValue}>
            {new Date(selectedYear, selectedMonth - 1).toLocaleString("en-US", {
              month: "long",
              year: "numeric",
            })}
          </option>
        )}
      </select>
    </div>
  );
}
