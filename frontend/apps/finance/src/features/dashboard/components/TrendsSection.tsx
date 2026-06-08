import { useState } from "react";
import type { TrendPoint } from "../../../types";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@gofin/ui/components/select";
import { ToggleGroup, ToggleGroupItem } from "@gofin/ui/components/toggle-group";
import { SpendingTrendChart } from "./widgets/SpendingTrendChart";
import { CategorySplitChart } from "./widgets/CategorySplitChart";

type TrendsChart = "monthly-spending" | "category-split";

interface TrendsSectionProps {
  trendData: TrendPoint[];
  trendMonths: 6 | 12;
  onToggle: (months: 6 | 12) => void;
  currency: string;
}

export function TrendsSection({
  trendData,
  trendMonths,
  onToggle,
  currency,
}: TrendsSectionProps) {
  const [selectedChart, setSelectedChart] = useState<TrendsChart>("monthly-spending");

  if (trendData.length === 0) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Select
          value={selectedChart}
          onValueChange={(value) => setSelectedChart(value as TrendsChart)}
        >
          <SelectTrigger aria-label="Select trend chart">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="monthly-spending">Monthly Spending</SelectItem>
            <SelectItem value="category-split">Category Split</SelectItem>
          </SelectContent>
        </Select>
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
      {selectedChart === "monthly-spending" && (
        <SpendingTrendChart data={trendData} currency={currency} />
      )}
      {selectedChart === "category-split" && (
        <CategorySplitChart data={trendData} />
      )}
    </div>
  );
}
