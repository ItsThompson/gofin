import { useState } from "react";
import { Link } from "react-router";
import type { BudgetPeriod } from "@gofin/core";
import { Button } from "@gofin/ui/components/button";
import { Card, CardContent } from "@gofin/ui/components/card";
import { History, ArrowLeft, Loader2 } from "lucide-react";
import { ActiveDashboard } from "../dashboard";
import { useHistoryData } from "./hooks/useHistoryData";
import { PeriodRow } from "./components/PeriodRow";

/**
 * History page: shows past budget periods with spent/surplus data.
 * Clicking a period shows a read-only dashboard view.
 */
export function HistoryFeature() {
  const { periods, loading } = useHistoryData();
  const [selectedPeriod, setSelectedPeriod] = useState<BudgetPeriod | null>(
    null,
  );

  if (selectedPeriod) {
    return (
      <div className="space-y-4">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setSelectedPeriod(null)}
        >
          <ArrowLeft className="size-4" />
          Back to History
        </Button>
        <ActiveDashboard period={selectedPeriod} readOnly />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex min-h-[300px] items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
        <span className="ml-2 text-muted-foreground">Loading history...</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <History className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">Budget History</h1>
      </div>

      {periods.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-muted-foreground">No budget periods yet.</p>
            <Button asChild className="mt-4">
              <Link to="/dashboard">Go to Dashboard</Link>
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {periods.map((row) => (
            <PeriodRow
              key={row.period.id}
              row={row}
              onSelect={() => setSelectedPeriod(row.period)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
