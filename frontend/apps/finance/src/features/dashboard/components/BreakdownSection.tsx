import { useState } from "react";
import type { TagSpending } from "@gofin/core";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@gofin/ui/components/select";
import { TagSpendingChart } from "./widgets/TagSpendingChart";
import { ExpenseFrecencyChart } from "./widgets/ExpenseFrecencyChart";
import type { ExpenseFrecencyDataState } from "../hooks/useExpenseFrecencyData";

export type BreakdownChart = "tag-spending" | "repeated-expenses";

interface BreakdownSectionProps {
  tagSpending: TagSpending[];
  expenseFrecencyData: ExpenseFrecencyDataState;
  currency: string;
  selectedChart?: BreakdownChart;
  onChartChange?: (chart: BreakdownChart) => void;
}

export function BreakdownSection({
  tagSpending,
  expenseFrecencyData,
  currency,
  selectedChart: selectedChartProp,
  onChartChange,
}: BreakdownSectionProps) {
  const [uncontrolledChart, setUncontrolledChart] = useState<BreakdownChart>("tag-spending");
  const selectedChart = selectedChartProp ?? uncontrolledChart;
  const handleChartChange = (chart: BreakdownChart) => {
    setUncontrolledChart(chart);
    onChartChange?.(chart);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Select
          value={selectedChart}
          onValueChange={(value) => handleChartChange(value as BreakdownChart)}
        >
          <SelectTrigger aria-label="Select breakdown chart" className="text-base">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="tag-spending" className="text-base">Spending by Tag</SelectItem>
            <SelectItem value="repeated-expenses" className="text-base">Repeated Expenses</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {selectedChart === "tag-spending" && (
        <TagSpendingChart tagSpending={tagSpending} currency={currency} />
      )}
      {selectedChart === "repeated-expenses" && (
        <ExpenseFrecencyChart {...expenseFrecencyData} />
      )}
    </div>
  );
}
