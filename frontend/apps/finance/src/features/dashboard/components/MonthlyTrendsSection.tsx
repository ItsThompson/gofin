import type { TrendPoint } from "../../../types";
import { ToggleGroup, ToggleGroupItem } from "@gofin/ui/components/toggle-group";
import { SpendingTrendChart } from "./widgets/SpendingTrendChart";
import { CategorySplitChart } from "./widgets/CategorySplitChart";

interface MonthlyTrendsSectionProps {
  trendData: TrendPoint[];
  trendMonths: 6 | 12;
  onToggle: (months: 6 | 12) => void;
  currency: string;
}

export function MonthlyTrendsSection({
  trendData,
  trendMonths,
  onToggle,
  currency,
}: MonthlyTrendsSectionProps) {
  if (trendData.length === 0) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold">Monthly Trends</h2>
        <ToggleGroup
          type="single"
          value={String(trendMonths)}
          onValueChange={(value) => {
            if (value === "6" || value === "12") {
              onToggle(Number(value) as 6 | 12);
            }
          }}
          size="sm"
        >
          <ToggleGroupItem value="6" aria-label="6 months">
            6M
          </ToggleGroupItem>
          <ToggleGroupItem value="12" aria-label="12 months">
            12M
          </ToggleGroupItem>
        </ToggleGroup>
      </div>
      <SpendingTrendChart data={trendData} currency={currency} />
      <CategorySplitChart data={trendData} />
    </div>
  );
}
