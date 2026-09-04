import { getCurrencySymbol } from "@gofin/core";
import type { FinancePageProps } from "../../types";
import { ExpenseForm } from "./components/ExpenseForm";
import { LoadingPeriodCard } from "./components/LoadingPeriodCard";
import { MissingPeriodCard } from "./components/MissingPeriodCard";
import { useNewExpenseForm } from "./hooks/useNewExpenseForm";
import { usePeriodContext } from "./hooks/usePeriodContext";

export function NewExpenseFeature({ user }: FinancePageProps) {
  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const periodContext = usePeriodContext(currentYear, currentMonth);
  const activePeriod =
    periodContext.status === "active" ? periodContext.period : null;
  const { state, actions } = useNewExpenseForm(
    activePeriod?.reportingCurrency ?? user.currency,
    activePeriod?.year ?? currentYear,
    activePeriod?.month ?? currentMonth,
  );
  const currencySymbol = getCurrencySymbol(state.transactionCurrencyCode);

  if (periodContext.status === "loading") {
    return <LoadingPeriodCard />;
  }

  if (
    periodContext.status === "missing" ||
    periodContext.status === "error"
  ) {
    return (
      <MissingPeriodCard
        year={currentYear}
        month={currentMonth}
        errorMessage={
          periodContext.status === "error" ? periodContext.message : undefined
        }
      />
    );
  }

  return (
    <ExpenseForm
      state={state}
      actions={actions}
      currencySymbol={currencySymbol}
      year={currentYear}
      month={currentMonth}
    />
  );
}
