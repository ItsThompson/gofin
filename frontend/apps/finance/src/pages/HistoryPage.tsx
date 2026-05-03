import { useState, useEffect } from "react";
import { Link, useNavigate } from "react-router";
import {
  apiClient,
  formatCurrency,
  type BudgetPeriod,
  type PeriodListResponse,
  type PeriodSummary,
  type SummaryResponse,
  type User,
} from "@gofin/types";
import { Button } from "@gofin/ui/components/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import { History, ArrowLeft, Loader2, ArrowRight } from "lucide-react";
import type { FinancePageProps } from "@/types";
import { ActiveDashboard } from "@/pages/DashboardPage";

interface HistoricalPeriodRow {
  period: BudgetPeriod;
  totalSpent: number;
  surplus: number;
}

/**
 * History page: shows past budget periods with spent/surplus data.
 * Clicking a period shows a read-only dashboard view.
 */
export function HistoryPage({ user }: FinancePageProps) {
  const [periods, setPeriods] = useState<HistoricalPeriodRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedPeriod, setSelectedPeriod] = useState<BudgetPeriod | null>(
    null,
  );

  useEffect(() => {
    async function fetchPeriods() {
      try {
        const periodsRes =
          await apiClient<PeriodListResponse>("/api/finance/periods");
        const allPeriods = periodsRes.periods;

        // Fetch summary for each period to get totalSpent
        const rows = await Promise.all(
          allPeriods.map(async (period) => {
            try {
              const summaryRes = await apiClient<SummaryResponse>(
                `/api/finance/summary?year=${period.year}&month=${period.month}`,
              );
              const totalSpent = summaryRes.summary.totalSpent;
              return {
                period,
                totalSpent,
                surplus: period.budgetAmount - totalSpent,
              };
            } catch {
              return { period, totalSpent: 0, surplus: period.budgetAmount };
            }
          }),
        );

        setPeriods(rows);
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to load periods";
        setError(message);
      } finally {
        setLoading(false);
      }
    }
    fetchPeriods();
  }, []);

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
        <ActiveDashboard period={selectedPeriod} user={user} readOnly />
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

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>{error}</CardDescription>
        </CardHeader>
      </Card>
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
          {periods.map((row) => {
            const monthName = new Date(
              row.period.year,
              row.period.month - 1,
            ).toLocaleString("en-US", { month: "long", year: "numeric" });
            const isSurplus = row.surplus >= 0;

            return (
              <Card
                key={row.period.id}
                className="cursor-pointer transition-colors hover:bg-muted/50"
                onClick={() => setSelectedPeriod(row.period)}
                data-testid={`period-row-${row.period.year}-${row.period.month}`}
              >
                <CardContent className="flex items-center justify-between px-4 py-3">
                  <div className="flex flex-col gap-0.5">
                    <span className="font-medium">{monthName}</span>
                    <span className="text-xs text-muted-foreground">
                      Budget: {formatCurrency(row.period.budgetAmount, user.currency)}{" "}
                      · E/D/S: {row.period.essentialsPercent}/{row.period.desiresPercent}/
                      {row.period.savingsPercent}
                    </span>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="text-right">
                      <p className="text-sm">
                        Spent: {formatCurrency(row.totalSpent, user.currency)}
                      </p>
                      <p
                        className={`text-sm font-semibold ${
                          isSurplus
                            ? "text-green-600 dark:text-green-400"
                            : "text-red-600 dark:text-red-400"
                        }`}
                      >
                        {isSurplus ? "Surplus" : "Deficit"}:{" "}
                        {formatCurrency(Math.abs(row.surplus), user.currency)}
                      </p>
                    </div>
                    <ArrowRight className="size-4 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
