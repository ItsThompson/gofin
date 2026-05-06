import { useState, useEffect, useCallback, type FormEvent } from "react";
import { Link, useNavigate } from "react-router";
import {
  apiClient,
  ApiRequestError,
  useApiToast,
  formatCurrency,
  getCurrencySymbol,
  type BudgetPeriod,
  type DefaultSettings,
  type DefaultsResponse,
  type PeriodResponse,
  type CreatePeriodRequest,
  type UpdatePeriodRequest,
  type Expense,
  type PaginatedResponse,
  type PeriodSummary,
  type SummaryResponse,
  type TagSpending,
  type TagSpendingResponse,
  type CumulativeSpendPoint,
  type CumulativeSpendResponse,
  type HistoricalComparison,
  type HistoricalComparisonResponse,
  type ProRataSchedule,
  type UpcomingProRataResponse,
  type CreatePeriodResponse,
  type TrendPoint,
  type TrendResponse,
  type User,
} from "@gofin/types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
  FormDescription,
} from "@gofin/ui/components/form";
import { SectionErrorBoundary } from "@gofin/ui/components/section-error-boundary";
import { DashboardSkeleton } from "@gofin/ui/components/skeletons";
import {
  LayoutDashboard,
  PlusCircle,
  Wallet,
  TrendingDown,
  TrendingUp,
  Calendar,
  AlertTriangle,
  Activity,
  Target,
  Settings2,
  History,
} from "lucide-react";
import {
  BarChart,
  Bar,
  Line,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ComposedChart,
} from "recharts";
import type { DashboardState, FinancePageProps } from "../types";
import { getRemainingColor } from "../lib/budget-utils";
import { MonthlyTrendsSection } from "./MonthlyTrendsSection";

/**
 * Dashboard page: the central finance view.
 *
 * On load, fetches the current month's budget period. If none exists,
 * shows a creation prompt with default values. After creation (or if
 * a period already exists), renders the full analytics dashboard.
 */
export function DashboardPage({ user }: FinancePageProps) {
  const [state, setState] = useState<DashboardState>("loading");
  const [period, setPeriod] = useState<BudgetPeriod | null>(null);
  const [defaults, setDefaults] = useState<DefaultSettings | null>(null);
  const { call: toastCall } = useApiToast();

  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const fetchPeriod = useCallback(async () => {
    setState("loading");
    try {
      const response = await apiClient<PeriodResponse>(
        `/api/finance/periods/current?year=${currentYear}&month=${currentMonth}`,
      );
      setPeriod(response.period);
      setState("active");
    } catch (error) {
      if (
        error instanceof ApiRequestError &&
        error.code === "PERIOD_NOT_FOUND"
      ) {
        try {
          const defaultsResponse =
            await apiClient<DefaultsResponse>("/api/finance/defaults");
          setDefaults(defaultsResponse.defaults);
        } catch {
          setDefaults(null);
        }
        setState("no-period");
        return;
      }
      // Use toast for non-domain errors
      await toastCall(() => Promise.reject(error));
      setState("error");
    }
  }, [currentYear, currentMonth, toastCall]);

  useEffect(() => {
    fetchPeriod();
  }, [fetchPeriod]);

  if (state === "loading") {
    return <DashboardSkeleton />;
  }

  if (state === "error") {
    return <ErrorState onRetry={fetchPeriod} />;
  }

  if (state === "no-period") {
    return (
      <CreatePeriodPrompt
        defaults={defaults}
        user={user}
        year={currentYear}
        month={currentMonth}
        onPeriodCreated={(newPeriod) => {
          setPeriod(newPeriod);
          setState("active");
        }}
      />
    );
  }

  return <ActiveDashboard period={period!} user={user} />;
}

// --- Sub-components ---

function ErrorState({
  onRetry,
}: {
  onRetry: () => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-destructive">Something went wrong</CardTitle>
        <CardDescription>
          Could not load the dashboard. The error details were shown in a
          notification.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button variant="outline" onClick={onRetry}>
          Retry
        </Button>
      </CardContent>
    </Card>
  );
}

// --- Period Creation Prompt ---

interface CreatePeriodPromptProps {
  defaults: DefaultSettings | null;
  user: User;
  year: number;
  month: number;
  onPeriodCreated: (period: BudgetPeriod) => void;
}

const FALLBACK_DEFAULTS = {
  budgetAmount: 0,
  essentialsPercent: 50,
  desiresPercent: 30,
  savingsPercent: 20,
};

function CreatePeriodPrompt({
  defaults,
  user,
  year,
  month,
  onPeriodCreated,
}: CreatePeriodPromptProps) {
  const effectiveDefaults = defaults ?? FALLBACK_DEFAULTS;
  const isZeroBudget = effectiveDefaults.budgetAmount === 0;
  const currencySymbol = getCurrencySymbol(user.currency);

  const [budgetDollars, setBudgetDollars] = useState<string>(
    effectiveDefaults.budgetAmount > 0
      ? (effectiveDefaults.budgetAmount / 100).toString()
      : "",
  );
  const [essentials, setEssentials] = useState<string>(
    String(effectiveDefaults.essentialsPercent),
  );
  const [desires, setDesires] = useState<string>(
    String(effectiveDefaults.desiresPercent),
  );
  const [savings, setSavings] = useState<string>(
    String(effectiveDefaults.savingsPercent),
  );
  const [splitError, setSplitError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const monthName = new Date(year, month - 1).toLocaleString("en-US", {
    month: "long",
  });

  function validateSplit(): string | null {
    const essentialsVal = parseInt(essentials, 10) || 0;
    const desiresVal = parseInt(desires, 10) || 0;
    const savingsVal = parseInt(savings, 10) || 0;
    const total = essentialsVal + desiresVal + savingsVal;
    if (total !== 100) {
      return `Percentages must sum to 100%. Currently: ${total}%`;
    }
    if (essentialsVal < 0 || desiresVal < 0 || savingsVal < 0) {
      return "Percentages must be non-negative";
    }
    return null;
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);

    const splitValidation = validateSplit();
    if (splitValidation) {
      setSplitError(splitValidation);
      return;
    }

    const budgetAmountCents = Math.round(
      (parseFloat(budgetDollars) || 0) * 100,
    );

    const body: CreatePeriodRequest = {
      year,
      month,
      budgetAmount: budgetAmountCents,
      essentialsPercent: parseInt(essentials, 10) || 0,
      desiresPercent: parseInt(desires, 10) || 0,
      savingsPercent: parseInt(savings, 10) || 0,
    };

    setSubmitting(true);
    try {
      const response = await apiClient<CreatePeriodResponse>("/api/finance/periods", {
        method: "POST",
        body: JSON.stringify(body),
      });

      // Show notification if missed months were auto-created
      if (response.autoCreatedPeriods && response.autoCreatedPeriods > 0 && response.autoCreatedMonths) {
        const monthsList = response.autoCreatedMonths.join(", ");
        alert(`${response.autoCreatedPeriods} period(s) were automatically created for ${monthsList} with your default settings.`);
      }

      onPeriodCreated(response.period);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred. Please try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  const splitTotal =
    (parseInt(essentials, 10) || 0) +
    (parseInt(desires, 10) || 0) +
    (parseInt(savings, 10) || 0);

  return (
    <div className="flex items-start justify-center pt-8">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle className="text-2xl">
            Set Up {monthName} {year}
          </CardTitle>
          <CardDescription>
            No budget period exists for this month. Confirm your settings or
            customize them below.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isZeroBudget && (
            <div className="mb-4 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <span>
                No budget configured yet. Set an amount below, or create the
                period at $0 and update it later in Settings.
              </span>
            </div>
          )}

          <Form onSubmit={handleSubmit}>
            <FormField>
              <FormLabel htmlFor="budget">Monthly Budget</FormLabel>
              <div className="relative">
                <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {currencySymbol}
                </span>
                <Input
                  id="budget"
                  type="number"
                  min="0"
                  step="0.01"
                  placeholder="0.00"
                  value={budgetDollars}
                  onChange={(event) => setBudgetDollars(event.target.value)}
                  className="pl-6"
                />
              </div>
            </FormField>

            <FormField>
              <FormLabel htmlFor="essentials">Essentials %</FormLabel>
              <Input
                id="essentials"
                type="number"
                min="0"
                max="100"
                value={essentials}
                onChange={(event) => {
                  setEssentials(event.target.value);
                  setSplitError(null);
                }}
                aria-invalid={!!splitError}
              />
            </FormField>

            <FormField>
              <FormLabel htmlFor="desires">Desires %</FormLabel>
              <Input
                id="desires"
                type="number"
                min="0"
                max="100"
                value={desires}
                onChange={(event) => {
                  setDesires(event.target.value);
                  setSplitError(null);
                }}
                aria-invalid={!!splitError}
              />
            </FormField>

            <FormField>
              <FormLabel htmlFor="savings">Savings %</FormLabel>
              <Input
                id="savings"
                type="number"
                min="0"
                max="100"
                value={savings}
                onChange={(event) => {
                  setSavings(event.target.value);
                  setSplitError(null);
                }}
                aria-invalid={!!splitError}
              />
            </FormField>

            <FormDescription>Total: {splitTotal}%</FormDescription>
            {splitError && <FormMessage>{splitError}</FormMessage>}
            {error && <FormMessage>{error}</FormMessage>}

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? "Creating..." : `Create ${monthName} Period`}
            </Button>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}

// --- Active Dashboard ---

export interface ActiveDashboardProps {
  period: BudgetPeriod;
  user: User;
  /** When true, hides editing controls and Log Expense CTA. */
  readOnly?: boolean;
}

export function ActiveDashboard({ period, user, readOnly = false }: ActiveDashboardProps) {
  const [summary, setSummary] = useState<PeriodSummary | null>(null);
  const [tagSpending, setTagSpending] = useState<TagSpending[]>([]);
  const [cumulativeData, setCumulativeData] = useState<
    CumulativeSpendPoint[]
  >([]);
  const [recentExpenses, setRecentExpenses] = useState<Expense[]>([]);
  const [comparison, setComparison] = useState<HistoricalComparison | null>(null);
  const [upcomingProRata, setUpcomingProRata] = useState<ProRataSchedule[]>([]);
  const [trendData, setTrendData] = useState<TrendPoint[] | null>(null);
  const [trendMonths, setTrendMonths] = useState<6 | 12>(6);
  const [dataLoaded, setDataLoaded] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [currentPeriod, setCurrentPeriod] = useState(period);
  const { call: toastCall } = useApiToast();

  const fetchDashboardData = useCallback(async () => {
    setDataLoaded(false);

    // Fetch each section independently so a single endpoint failure
    // doesn't prevent the rest of the dashboard from rendering.
    const [summaryRes, tagRes, cumulativeRes, expensesRes, comparisonRes, upcomingRes] =
      await Promise.all([
        toastCall(() =>
          apiClient<SummaryResponse>(
            `/api/finance/summary?year=${currentPeriod.year}&month=${currentPeriod.month}`,
          ),
        ),
        toastCall(() =>
          apiClient<TagSpendingResponse>(
            `/api/finance/spending/by-tag?year=${currentPeriod.year}&month=${currentPeriod.month}`,
          ),
        ),
        toastCall(() =>
          apiClient<CumulativeSpendResponse>(
            `/api/finance/spending/cumulative?year=${currentPeriod.year}&month=${currentPeriod.month}`,
          ),
        ),
        toastCall(() =>
          apiClient<PaginatedResponse<Expense>>(
            `/api/expenses?year=${currentPeriod.year}&month=${currentPeriod.month}&page=1&pageSize=5`,
          ),
        ),
        apiClient<HistoricalComparisonResponse>(
          `/api/finance/spending/comparison?year=${currentPeriod.year}&month=${currentPeriod.month}`,
        ).catch(() => null),
        apiClient<UpcomingProRataResponse>(
          `/api/finance/prorata/upcoming`,
        ).catch(() => ({ schedules: [] as ProRataSchedule[] })),
      ]);

    if (summaryRes) setSummary(summaryRes.summary);
    if (tagRes) setTagSpending(tagRes.tagSpending);
    if (cumulativeRes) setCumulativeData(cumulativeRes.points);
    if (expensesRes) setRecentExpenses(expensesRes.data);
    if (comparisonRes) setComparison(comparisonRes.comparison);
    if (upcomingRes) setUpcomingProRata(upcomingRes.schedules);

    setDataLoaded(true);
  }, [currentPeriod.year, currentPeriod.month, toastCall]);

  const fetchTrendData = useCallback(async () => {
    const trendRes = await apiClient<TrendResponse>(
      `/api/finance/spending/trends?year=${currentPeriod.year}&month=${currentPeriod.month}&months=${trendMonths}`,
    ).catch(() => null);
    setTrendData(trendRes?.trends ?? null);
  }, [currentPeriod.year, currentPeriod.month, trendMonths]);

  useEffect(() => {
    fetchDashboardData();
  }, [fetchDashboardData]);

  useEffect(() => {
    fetchTrendData();
  }, [fetchTrendData]);

  function handlePeriodUpdated(updatedPeriod: BudgetPeriod) {
    setCurrentPeriod(updatedPeriod);
    setShowSettings(false);
    fetchDashboardData();
  }

  const totalSpent = summary?.totalSpent ?? 0;
  const remaining = summary?.remaining ?? currentPeriod.budgetAmount;

  const monthName = new Date(currentPeriod.year, currentPeriod.month - 1).toLocaleString("en-US", {
    month: "long",
    year: "numeric",
  });

  if (!dataLoaded) {
    return <DashboardSkeleton />;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <LayoutDashboard className="size-6 text-primary" />
          <h1 className="text-2xl font-bold">
            {readOnly ? monthName : "Dashboard"}
          </h1>
        </div>
        <div className="flex items-center gap-2">
          {!readOnly && (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowSettings(!showSettings)}
                aria-label="Budget Settings"
              >
                <Settings2 className="size-4" />
                <span className="hidden sm:inline ml-1">Budget Settings</span>
              </Button>
              {/* Desktop-only Log Expense button (FAB handles mobile) */}
              <Button asChild className="hidden md:inline-flex">
                <Link to="/expenses/new">
                  <PlusCircle className="size-4" />
                  Log Expense
                </Link>
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Budget Settings Editor */}
      {showSettings && !readOnly && (
        <SectionErrorBoundary sectionName="Budget Settings">
          <BudgetSettingsEditor
            period={currentPeriod}
            currency={user.currency}
            onSaved={handlePeriodUpdated}
            onCancel={() => setShowSettings(false)}
          />
        </SectionErrorBoundary>
      )}

      {/* Summary Bar */}
      <SectionErrorBoundary sectionName="Summary">
        <SummaryBar
        budgetAmount={currentPeriod.budgetAmount}
        totalSpent={totalSpent}
        remaining={remaining}
        daysLeft={
          summary
            ? summary.daysInPeriod - summary.daysElapsed
            : Math.max(
                0,
                new Date(currentPeriod.year, currentPeriod.month, 0).getDate() -
                  new Date().getDate(),
              )
        }
        currency={user.currency}
      />
      </SectionErrorBoundary>

      {/* Category Gauges */}
      <SectionErrorBoundary sectionName="Category Gauges">
        {summary && <CategoryGauges summary={summary} currency={user.currency} />}
      </SectionErrorBoundary>

      {/* Pacing Indicator */}
      <SectionErrorBoundary sectionName="Spending Pace">
        {summary && <PacingIndicator summary={summary} currency={user.currency} />}
      </SectionErrorBoundary>

      {/* Upcoming Pro-rata */}
      <SectionErrorBoundary sectionName="Upcoming Pro-rata">
        {upcomingProRata.length > 0 && (
          <UpcomingProRataSection schedules={upcomingProRata} currency={user.currency} />
        )}
      </SectionErrorBoundary>

      {/* Charts: hidden on mobile per US-DASH-09 */}
      <div className="hidden md:block space-y-6">
        {/* Historical Comparison Widget */}
        <SectionErrorBoundary sectionName="Historical Comparison">
          {comparison && (
            <HistoricalComparisonWidget
              comparison={comparison}
              currency={user.currency}
            />
          )}
        </SectionErrorBoundary>

        {/* Monthly Trends Section */}
        <SectionErrorBoundary sectionName="Monthly Trends">
          {trendData && trendData.length > 0 && (
            <MonthlyTrendsSection
              trendData={trendData}
              trendMonths={trendMonths}
              onToggle={setTrendMonths}
              currency={user.currency}
            />
          )}
        </SectionErrorBoundary>

        {/* Tag Spending Chart */}
        <SectionErrorBoundary sectionName="Tag Spending">
          {tagSpending.length > 0 && (
            <TagSpendingChart tagSpending={tagSpending} currency={user.currency} />
          )}
        </SectionErrorBoundary>

        {/* Cumulative Spend Chart */}
        <SectionErrorBoundary sectionName="Cumulative Spending">
          {cumulativeData.length > 0 && (
            <CumulativeSpendChart
              data={cumulativeData}
              currency={user.currency}
            />
          )}
        </SectionErrorBoundary>
      </div>

      {/* Recent Expenses or Empty State */}
      <SectionErrorBoundary sectionName="Recent Expenses">
      {dataLoaded && recentExpenses.length === 0 && !readOnly ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <Wallet className="mb-4 size-12 text-muted-foreground/50" />
            <h2 className="mb-2 text-lg font-semibold">No expenses yet</h2>
            <p className="mb-6 max-w-sm text-sm text-muted-foreground">
              Start tracking your spending by logging your first expense for this
              month.
            </p>
            <Button asChild>
              <Link to="/expenses/new">
                <PlusCircle className="size-4" />
                Log your first expense
              </Link>
            </Button>
          </CardContent>
        </Card>
      ) : dataLoaded && recentExpenses.length > 0 ? (
        <RecentExpenses expenses={recentExpenses} currency={user.currency} />
      ) : null}
      </SectionErrorBoundary>
    </div>
  );
}

// --- Category Gauges ---

interface CategoryGaugesProps {
  summary: PeriodSummary;
  currency: string;
}

function CategoryGauges({ summary, currency }: CategoryGaugesProps) {
  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
      <CategoryGauge
        label="Essentials"
        category={summary.essentials}
        currency={currency}
      />
      <CategoryGauge
        label="Desires"
        category={summary.desires}
        currency={currency}
      />
      <CategoryGauge
        label="Savings"
        category={summary.savings}
        currency={currency}
      />
    </div>
  );
}

function CategoryGauge({
  label,
  category,
  currency,
}: {
  label: string;
  category: PeriodSummary["essentials"];
  currency: string;
}) {
  const isOverBudget = category.percentUsed >= 100;
  const progressPercent = Math.min(category.percentUsed, 100);

  return (
    <Card data-testid={`gauge-${label.toLowerCase()}`}>
      <CardContent className="px-4 py-3">
        <div className="flex items-center justify-between text-sm">
          <span className="font-medium">{label}</span>
          <span
            className={`text-xs font-semibold ${isOverBudget ? "text-red-600 dark:text-red-400" : "text-muted-foreground"}`}
          >
            {category.percentUsed.toFixed(0)}%
          </span>
        </div>
        {/* Progress bar */}
        <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-muted">
          <div
            className={`h-full rounded-full transition-all ${isOverBudget ? "bg-red-500" : "bg-primary"}`}
            style={{ width: `${progressPercent}%` }}
            role="progressbar"
            aria-valuenow={category.percentUsed}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`${label} budget usage`}
          />
        </div>
        <div className="mt-2 flex justify-between text-xs text-muted-foreground">
          <span>
            {formatCurrency(category.spent, currency)} of{" "}
            {formatCurrency(category.allocated, currency)}
          </span>
          <span>
            {isOverBudget ? (
              <span className="text-red-600 dark:text-red-400">
                Over by {formatCurrency(Math.abs(category.remaining), currency)}
              </span>
            ) : (
              <span>
                {formatCurrency(category.remaining, currency)} left
              </span>
            )}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}

// --- Pacing Indicator ---

interface PacingIndicatorProps {
  summary: PeriodSummary;
  currency: string;
}

function PacingIndicator({ summary, currency }: PacingIndicatorProps) {
  const isOverBudget = summary.totalSpent > summary.totalBudget;
  const overAmount = isOverBudget ? summary.totalSpent - summary.totalBudget : 0;

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <Activity className="size-4 text-muted-foreground" />
          <CardTitle className="text-base">Spending Pace</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div>
            <p className="text-xs text-muted-foreground">Daily Average</p>
            <p className="text-lg font-semibold">
              {formatCurrency(summary.dailySpendRate, currency)}/day
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Required Rate</p>
            <p className="text-lg font-semibold">
              {summary.budgetPace > 0
                ? `${formatCurrency(summary.budgetPace, currency)}/day`
                : "N/A"}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Status</p>
            {isOverBudget ? (
              <div className="flex items-center gap-1">
                <TrendingUp className="size-4 text-red-600 dark:text-red-400" />
                <p className="text-lg font-semibold text-red-600 dark:text-red-400">
                  Over by {formatCurrency(overAmount, currency)}
                </p>
              </div>
            ) : summary.isOnTrack ? (
              <div className="flex items-center gap-1">
                <Target className="size-4 text-green-600 dark:text-green-400" />
                <p className="text-lg font-semibold text-green-600 dark:text-green-400">
                  On Track
                </p>
              </div>
            ) : (
              <div className="flex items-center gap-1">
                <AlertTriangle className="size-4 text-yellow-600 dark:text-yellow-400" />
                <p className="text-lg font-semibold text-yellow-600 dark:text-yellow-400">
                  Over Pace
                </p>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

// --- Upcoming Pro-rata Section ---

interface UpcomingProRataSectionProps {
  schedules: ProRataSchedule[];
  currency: string;
}

function UpcomingProRataSection({ schedules, currency }: UpcomingProRataSectionProps) {
  return (
    <Card data-testid="upcoming-prorata">
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <Calendar className="size-4 text-muted-foreground" />
          <CardTitle className="text-base">Upcoming Pro-rata</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="divide-y">
          {schedules.map((schedule) => (
            <div
              key={schedule.id}
              className="flex items-center justify-between py-3 first:pt-0 last:pb-0"
            >
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-medium">{schedule.name}</span>
                <span className="text-xs text-muted-foreground">
                  Installment {schedule.installmentIndex} of {schedule.installmentTotal}
                </span>
              </div>
              <span className="text-sm font-semibold">
                {formatCurrency(schedule.amount, currency)}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

// --- Tag Spending Bar Chart ---

interface TagSpendingChartProps {
  tagSpending: TagSpending[];
  currency: string;
}

function TagSpendingChart({ tagSpending, currency }: TagSpendingChartProps) {
  const navigate = useNavigate();

  const chartData = tagSpending.map((tag) => ({
    name: tag.tagName,
    amount: tag.amount / 100,
    tagId: tag.tagId,
    percent: tag.percentOfTotal,
  }));

  function handleBarClick(data: { tagId?: string }) {
    if (data.tagId) {
      navigate(`/expenses?tag=${data.tagId}`);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Spending by Tag</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={Math.max(200, tagSpending.length * 40)}>
          <BarChart
            data={chartData}
            layout="vertical"
            margin={{ top: 0, right: 20, left: 10, bottom: 0 }}
          >
            <CartesianGrid strokeDasharray="3 3" horizontal={false} />
            <XAxis type="number" tickFormatter={(value) => `${getCurrencySymbol(currency)}${value}`} />
            <YAxis type="category" dataKey="name" width={100} tick={{ fontSize: 12 }} />
            <Tooltip
              formatter={(value: number, _name: string, props: { payload: { percent: number } }) => [
                `${formatCurrency(value * 100, currency)} (${props.payload.percent.toFixed(1)}%)`,
                "Spent",
              ]}
            />
            <Bar
              dataKey="amount"
              fill="var(--primary)"
              radius={[0, 4, 4, 0]}
              cursor="pointer"
              onClick={(_data: unknown, index: number) => handleBarClick(chartData[index])}
            />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}

// --- Cumulative Spend Line Chart ---

interface CumulativeSpendChartProps {
  data: CumulativeSpendPoint[];
  currency: string;
}

function CumulativeSpendChart({ data, currency }: CumulativeSpendChartProps) {
  // Compute chart data with area-between-lines shading.
  // surplusBase/surplusTop fill green between ideal and actual when under budget.
  // deficitBase/deficitTop fill red between actual and ideal when over budget.
  const chartData = data.map((point) => {
    const actual = point.actual / 100;
    const ideal = point.ideal / 100;
    const underBudget = actual <= ideal;
    return {
      day: point.day,
      actual,
      ideal,
      surplusBase: underBudget ? actual : undefined,
      surplusTop: underBudget ? ideal : undefined,
      deficitBase: !underBudget ? ideal : undefined,
      deficitTop: !underBudget ? actual : undefined,
    };
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Cumulative Spending</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={300}>
          <ComposedChart
            data={chartData}
            margin={{ top: 5, right: 20, left: 10, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis
              dataKey="day"
              label={{ value: "Day of Month", position: "insideBottom", offset: -5 }}
            />
            <YAxis
              tickFormatter={(value) => `${getCurrencySymbol(currency)}${value}`}
            />
            <Tooltip
              formatter={(value: number) => formatCurrency(value * 100, currency)}
            />
            <Area
              type="monotone"
              dataKey="surplusTop"
              fill="rgba(34, 197, 94, 0.50)"
              stroke="none"
              name="Under Budget"
              connectNulls={false}
              isAnimationActive={false}
            />
            <Area
              type="monotone"
              dataKey="deficitTop"
              fill="rgba(239, 68, 68, 0.50)"
              stroke="none"
              name="Over Budget"
              connectNulls={false}
              isAnimationActive={false}
            />
            <Line
              type="monotone"
              dataKey="ideal"
              stroke="var(--muted-foreground)"
              strokeDasharray="5 5"
              strokeWidth={2}
              dot={false}
              name="Budget Pace"
            />
            <Line
              type="monotone"
              dataKey="actual"
              stroke="var(--primary)"
              strokeWidth={2}
              dot={false}
              name="Actual"
            />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}

// --- Recent Expenses ---

interface RecentExpensesProps {
  expenses: Expense[];
  currency: string;
}

function RecentExpenses({ expenses, currency }: RecentExpensesProps) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Recent Expenses</CardTitle>
          <Link
            to="/expenses"
            className="text-sm text-primary hover:underline"
          >
            View All
          </Link>
        </div>
      </CardHeader>
      <CardContent>
        <div className="divide-y">
          {expenses.map((expense) => (
            <div
              key={expense.id}
              className="flex items-center justify-between py-3 first:pt-0 last:pb-0"
            >
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-medium">{expense.name}</span>
                <span className="text-xs text-muted-foreground">
                  {expense.expenseDate}
                </span>
              </div>
              <span className="text-sm font-semibold">
                {formatCurrency(expense.amount, currency)}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

// --- Summary Bar ---

interface SummaryBarProps {
  budgetAmount: number;
  totalSpent: number;
  remaining: number;
  daysLeft: number;
  currency: string;
}

function SummaryBar({
  budgetAmount,
  totalSpent,
  remaining,
  daysLeft,
  currency,
}: SummaryBarProps) {
  const remainingColor = getRemainingColor(budgetAmount, remaining);

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      <SummaryCard
        label="Total Budget"
        value={formatCurrency(budgetAmount, currency)}
        icon={<Wallet className="size-4 text-muted-foreground" />}
      />
      <SummaryCard
        label="Total Spent"
        value={formatCurrency(totalSpent, currency)}
        icon={<TrendingDown className="size-4 text-muted-foreground" />}
      />
      <SummaryCard
        label="Remaining"
        value={formatCurrency(remaining, currency)}
        icon={<Wallet className="size-4 text-muted-foreground" />}
        valueClassName={remainingColor}
      />
      <SummaryCard
        label="Days Left"
        value={String(daysLeft)}
        icon={<Calendar className="size-4 text-muted-foreground" />}
      />
    </div>
  );
}

function SummaryCard({
  label,
  value,
  icon,
  valueClassName,
}: {
  label: string;
  value: string;
  icon: React.ReactNode;
  valueClassName?: string;
}) {
  return (
    <Card>
      <CardContent className="px-4 py-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {icon}
          {label}
        </div>
        <p className={`mt-1 text-xl font-bold ${valueClassName ?? ""}`}>
          {value}
        </p>
      </CardContent>
    </Card>
  );
}

// --- Budget Settings Editor ---

interface BudgetSettingsEditorProps {
  period: BudgetPeriod;
  currency: string;
  onSaved: (period: BudgetPeriod) => void;
  onCancel: () => void;
}

function BudgetSettingsEditor({
  period,
  currency,
  onSaved,
  onCancel,
}: BudgetSettingsEditorProps) {
  const currencySymbol = getCurrencySymbol(currency);
  const [budgetDollars, setBudgetDollars] = useState(
    (period.budgetAmount / 100).toString(),
  );
  const [essentials, setEssentials] = useState(String(period.essentialsPercent));
  const [desires, setDesires] = useState(String(period.desiresPercent));
  const [savings, setSavings] = useState(String(period.savingsPercent));
  const [splitError, setSplitError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const splitTotal =
    (parseInt(essentials, 10) || 0) +
    (parseInt(desires, 10) || 0) +
    (parseInt(savings, 10) || 0);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);

    const essentialsVal = parseInt(essentials, 10) || 0;
    const desiresVal = parseInt(desires, 10) || 0;
    const savingsVal = parseInt(savings, 10) || 0;
    const total = essentialsVal + desiresVal + savingsVal;

    if (total !== 100) {
      setSplitError(`Percentages must sum to 100%. Currently: ${total}%`);
      return;
    }

    const budgetAmountCents = Math.round((parseFloat(budgetDollars) || 0) * 100);

    const body: UpdatePeriodRequest = {
      budgetAmount: budgetAmountCents,
      essentialsPercent: essentialsVal,
      desiresPercent: desiresVal,
      savingsPercent: savingsVal,
    };

    setSubmitting(true);
    try {
      const response = await apiClient<PeriodResponse>(
        `/api/finance/periods/${period.id}`,
        {
          method: "PUT",
          body: JSON.stringify(body),
        },
      );
      onSaved(response.period);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred. Please try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Card data-testid="budget-settings-editor">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Settings2 className="size-4 text-muted-foreground" />
            <CardTitle className="text-base">Budget Settings</CardTitle>
          </div>
          <Button variant="ghost" size="sm" onClick={onCancel}>
            Cancel
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <Form onSubmit={handleSubmit}>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
            <FormField>
              <FormLabel htmlFor="edit-budget">Monthly Budget</FormLabel>
              <div className="relative">
                <span className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {currencySymbol}
                </span>
                <Input
                  id="edit-budget"
                  type="number"
                  min="0"
                  step="0.01"
                  value={budgetDollars}
                  onChange={(event) => setBudgetDollars(event.target.value)}
                  className="pl-6"
                />
              </div>
            </FormField>
            <FormField>
              <FormLabel htmlFor="edit-essentials">Essentials %</FormLabel>
              <Input
                id="edit-essentials"
                type="number"
                min="0"
                max="100"
                value={essentials}
                onChange={(event) => {
                  setEssentials(event.target.value);
                  setSplitError(null);
                }}
              />
            </FormField>
            <FormField>
              <FormLabel htmlFor="edit-desires">Desires %</FormLabel>
              <Input
                id="edit-desires"
                type="number"
                min="0"
                max="100"
                value={desires}
                onChange={(event) => {
                  setDesires(event.target.value);
                  setSplitError(null);
                }}
              />
            </FormField>
            <FormField>
              <FormLabel htmlFor="edit-savings">Savings %</FormLabel>
              <Input
                id="edit-savings"
                type="number"
                min="0"
                max="100"
                value={savings}
                onChange={(event) => {
                  setSavings(event.target.value);
                  setSplitError(null);
                }}
              />
            </FormField>
          </div>
          <div className="mt-3 flex items-center justify-between">
            <FormDescription>Total: {splitTotal}%</FormDescription>
            <Button type="submit" size="sm" disabled={submitting}>
              {submitting ? "Saving..." : "Save Changes"}
            </Button>
          </div>
          {splitError && <FormMessage>{splitError}</FormMessage>}
          {error && <FormMessage>{error}</FormMessage>}
        </Form>
      </CardContent>
    </Card>
  );
}

// --- Historical Comparison Widget ---

interface HistoricalComparisonWidgetProps {
  comparison: HistoricalComparison;
  currency: string;
}

function HistoricalComparisonWidget({
  comparison,
  currency,
}: HistoricalComparisonWidgetProps) {
  const hasPrevious = comparison.previousSpent > 0 || comparison.currentSpent > 0;
  const isOnlyOnePeriod = comparison.previousSpent === 0 && comparison.changePercent === 0;

  return (
    <Card data-testid="historical-comparison">
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <History className="size-4 text-muted-foreground" />
          <CardTitle className="text-base">Historical Comparison</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        {isOnlyOnePeriod ? (
          <p className="text-sm text-muted-foreground">
            Not enough data for comparison
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div>
              <p className="text-xs text-muted-foreground">Current Period</p>
              <p className="text-lg font-semibold">
                {formatCurrency(comparison.currentSpent, currency)}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Previous Period</p>
              <p className="text-lg font-semibold">
                {formatCurrency(comparison.previousSpent, currency)}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">
                {comparison.rollingAverage != null ? "Rolling Average (3mo)" : "Change"}
              </p>
              {comparison.rollingAverage != null ? (
                <p className="text-lg font-semibold">
                  {formatCurrency(comparison.rollingAverage, currency)}
                </p>
              ) : null}
              {hasPrevious && (
                <div className="flex items-center gap-1 mt-1">
                  {comparison.changePercent > 0 ? (
                    <TrendingUp className="size-4 text-red-600 dark:text-red-400" />
                  ) : comparison.changePercent < 0 ? (
                    <TrendingDown className="size-4 text-green-600 dark:text-green-400" />
                  ) : null}
                  <span
                    className={`text-sm font-medium ${
                      comparison.changePercent > 0
                        ? "text-red-600 dark:text-red-400"
                        : comparison.changePercent < 0
                          ? "text-green-600 dark:text-green-400"
                          : "text-muted-foreground"
                    }`}
                  >
                    {comparison.changePercent > 0 ? "+" : ""}
                    {comparison.changePercent.toFixed(1)}% from last period
                  </span>
                </div>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
