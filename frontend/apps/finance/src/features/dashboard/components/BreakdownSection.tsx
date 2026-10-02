import { useState } from "react";
import type { TagSpending } from "@gofin/core";
import type {
  BreakdownChart,
  ExpenseSuggestionsState,
  SectionState,
} from "../types";
import { SectionState as SectionStateView } from "./SectionState";
export type { BreakdownChart } from "../types";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@gofin/ui/components/select";
import { TagSpendingChart } from "./widgets/TagSpendingChart";
import { ExpenseFrecencyChart } from "./widgets/ExpenseFrecencyChart";

interface BreakdownSectionProps {
  tagSpending: TagSpending[];
  tagSpendingState?: SectionState<readonly TagSpending[]>;
  expenseFrecencyData: ExpenseSuggestionsState;
  currency: string;
  onSuggestionsRetry?: () => void;
  selectedChart?: BreakdownChart;
  onChartChange?: (chart: BreakdownChart) => void;
  onTagSpendingRetry?: () => void;
}

export function BreakdownSection({
  tagSpending,
  expenseFrecencyData,
  currency,
  selectedChart: selectedChartProp,
  onChartChange,
  onSuggestionsRetry,
  onTagSpendingRetry,
  tagSpendingState,
}: BreakdownSectionProps) {
  const [uncontrolledChart, setUncontrolledChart] =
    useState<BreakdownChart>("tag-spending");
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
          onValueChange={(value) => {
            if (value === "tag-spending" || value === "repeated-expenses") {
              handleChartChange(value);
            }
          }}
        >
          <SelectTrigger
            aria-label="Select breakdown chart"
            className="text-base"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="tag-spending" className="text-base">
              Spending by Tag
            </SelectItem>
            <SelectItem value="repeated-expenses" className="text-base">
              Repeated Expenses
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      {selectedChart === "tag-spending" &&
        (tagSpendingState ? (
          <SectionStateView
            label="Tag spending"
            state={tagSpendingState}
            onRetry={onTagSpendingRetry ?? (() => undefined)}
            emptyMessage="No tag spending is available."
          >
            {(spending) => (
              <TagSpendingChart
                tagSpending={[...spending]}
                currency={currency}
              />
            )}
          </SectionStateView>
        ) : (
          <TagSpendingChart tagSpending={tagSpending} currency={currency} />
        ))}
      {selectedChart === "repeated-expenses" && (
        <ExpenseFrecencyChart
          {...expenseFrecencyData}
          onRetry={onSuggestionsRetry}
        />
      )}
    </div>
  );
}
