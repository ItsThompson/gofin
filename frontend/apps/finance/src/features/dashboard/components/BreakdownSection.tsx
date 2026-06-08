import { useState } from "react";
import type { TagSpending } from "../../../types";
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

type BreakdownChart = "tag-spending" | "repeated-expenses";

interface BreakdownSectionProps {
  tagSpending: TagSpending[];
  expenseFrecencyData: ExpenseFrecencyDataState;
  currency: string;
}

export function BreakdownSection({
  tagSpending,
  expenseFrecencyData,
  currency,
}: BreakdownSectionProps) {
  const [selectedChart, setSelectedChart] = useState<BreakdownChart>("tag-spending");

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Select
          value={selectedChart}
          onValueChange={(value) => setSelectedChart(value as BreakdownChart)}
        >
          <SelectTrigger aria-label="Select breakdown chart">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="tag-spending">Spending by Tag</SelectItem>
            <SelectItem value="repeated-expenses">Repeated Expenses</SelectItem>
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
