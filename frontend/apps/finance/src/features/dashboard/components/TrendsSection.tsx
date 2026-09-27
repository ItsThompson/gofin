import { useState } from "react";
import type { TrendPoint } from "@gofin/core";
import type { SectionState } from "../types";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@gofin/ui/components/select";
import { ToggleGroup, ToggleGroupItem } from "@gofin/ui/components/toggle-group";
import { Card, CardContent } from "@gofin/ui/components/card";
import { SpendingTrendChart } from "./widgets/SpendingTrendChart";
import { CategorySplitChart } from "./widgets/CategorySplitChart";
import { SectionState as SectionStateView } from "./SectionState";

type TrendsChart = "monthly-spending" | "category-split";

interface TrendsSectionProps {
  trendData: TrendPoint[];
  trendMonths: 6 | 12;
  onToggle: (months: 6 | 12) => void;
  currency: string;
  trendState?: SectionState<readonly TrendPoint[]>;
  onRetry?: () => void;
}

export function TrendsSection({
  trendData,
  trendMonths,
  onToggle,
  currency,
  trendState,
  onRetry,
}: TrendsSectionProps) {
  const [selectedChart, setSelectedChart] = useState<TrendsChart>("monthly-spending");

  const renderChart = (data: readonly TrendPoint[]) => {
    if (data.length === 0) {
      return (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            No trend data is available.
          </CardContent>
        </Card>
      );
    }
    if (selectedChart === "monthly-spending") {
      return <SpendingTrendChart data={[...data]} currency={currency} />;
    }
    return <CategorySplitChart data={[...data]} />;
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Select
          value={selectedChart}
          onValueChange={(value) => setSelectedChart(value as TrendsChart)}
        >
          <SelectTrigger aria-label="Select trend chart" className="text-base">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="monthly-spending" className="text-base">Monthly Spending</SelectItem>
            <SelectItem value="category-split" className="text-base">Category Split</SelectItem>
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
      {trendState ? (
        <SectionStateView
          label="Trends"
          state={trendState}
          onRetry={onRetry ?? (() => undefined)}
          emptyMessage="No trend data is available."
        >
          {(data) => renderChart(data)}
        </SectionStateView>
      ) : renderChart(trendData)}
    </div>
  );
}
