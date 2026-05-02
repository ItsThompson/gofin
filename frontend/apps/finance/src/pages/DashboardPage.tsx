import { useState, useEffect, useCallback, type FormEvent } from "react";
import { Link } from "react-router";
import {
  apiClient,
  ApiRequestError,
  formatCurrency,
  getCurrencySymbol,
  type BudgetPeriod,
  type DefaultSettings,
  type DefaultsResponse,
  type PeriodResponse,
  type CreatePeriodRequest,
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
import {
  LayoutDashboard,
  Loader2,
  PlusCircle,
  Wallet,
  TrendingDown,
  Calendar,
  AlertTriangle,
} from "lucide-react";
import type { DashboardState, FinancePageProps } from "@/types";
import { getRemainingColor } from "@/lib/budget-utils";

/**
 * Dashboard page: the central finance view.
 *
 * On load, fetches the current month's budget period. If none exists,
 * shows a creation prompt with default values. After creation (or if
 * a period already exists), renders the summary bar and empty state.
 *
 * Exported via Module Federation for the shell to load dynamically.
 */
export function DashboardPage({ user }: FinancePageProps) {
  const [state, setState] = useState<DashboardState>("loading");
  const [period, setPeriod] = useState<BudgetPeriod | null>(null);
  const [defaults, setDefaults] = useState<DefaultSettings | null>(null);
  const [errorMessage, setErrorMessage] = useState("");

  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const fetchPeriod = useCallback(async () => {
    setState("loading");
    setErrorMessage("");
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
        // Expected: no period for this month yet. Fetch defaults for the prompt.
        try {
          const defaultsResponse =
            await apiClient<DefaultsResponse>("/api/finance/defaults");
          setDefaults(defaultsResponse.defaults);
        } catch {
          // Defaults not found is non-fatal: use hardcoded fallbacks
          setDefaults(null);
        }
        setState("no-period");
        return;
      }
      const message =
        error instanceof Error ? error.message : "Failed to load dashboard";
      setErrorMessage(message);
      setState("error");
    }
  }, [currentYear, currentMonth]);

  useEffect(() => {
    fetchPeriod();
  }, [fetchPeriod]);

  if (state === "loading") {
    return <LoadingState />;
  }

  if (state === "error") {
    return <ErrorState message={errorMessage} onRetry={fetchPeriod} />;
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

  // state === "active"
  return <ActiveDashboard period={period!} user={user} />;
}

// --- Sub-components ---

function LoadingState() {
  return (
    <div className="flex min-h-[300px] items-center justify-center">
      <Loader2 className="size-6 animate-spin text-muted-foreground" />
      <span className="ml-2 text-muted-foreground">Loading dashboard...</span>
    </div>
  );
}

function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-destructive">Error</CardTitle>
        <CardDescription>{message}</CardDescription>
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

    const budgetAmountCents = Math.round((parseFloat(budgetDollars) || 0) * 100);

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
      const response = await apiClient<PeriodResponse>("/api/finance/periods", {
        method: "POST",
        body: JSON.stringify(body),
      });
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

interface ActiveDashboardProps {
  period: BudgetPeriod;
  user: User;
}

function ActiveDashboard({ period, user }: ActiveDashboardProps) {
  const totalSpent = 0; // No expenses yet: wired in a later ticket
  const remaining = period.budgetAmount - totalSpent;
  const daysInMonth = new Date(period.year, period.month, 0).getDate();
  const today = new Date();
  const daysElapsed =
    period.year === today.getFullYear() && period.month === today.getMonth() + 1
      ? today.getDate()
      : daysInMonth;
  const daysLeft = Math.max(0, daysInMonth - daysElapsed);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <LayoutDashboard className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">Dashboard</h1>
      </div>

      {/* Summary Bar */}
      <SummaryBar
        budgetAmount={period.budgetAmount}
        totalSpent={totalSpent}
        remaining={remaining}
        daysLeft={daysLeft}
        currency={user.currency}
      />

      {/* Empty State */}
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
    </div>
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

