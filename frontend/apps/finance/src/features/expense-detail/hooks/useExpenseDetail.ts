import { useState, useCallback, useEffect, useRef } from "react";
import { useFormMutation } from "@gofin/api";
import type { Expense, CorrectExpenseRequest, ExpenseResponse } from "../../../types";
import { expenseDetailApi } from "../api";
import type {
  ExpenseDetailState,
  UseExpenseDetailOptions,
  CorrectionState,
} from "../types";

export type { ExpenseDetailState, UseExpenseDetailOptions } from "../types";

export function useExpenseDetail(
  expenseId: string | null,
  options?: UseExpenseDetailOptions,
): ExpenseDetailState {
  const [expense, setExpense] = useState<Expense | null>(null);
  const [history, setHistory] = useState<Expense[]>([]);
  const [proRataGroup, setProRataGroup] = useState<Expense[]>([]);
  const [status, setStatus] = useState<"loading" | "detail" | "correct" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  const fetchExpenseData = useCallback(async () => {
    if (!expenseId) return;
    setStatus("loading");
    setError(null);

    try {
      const [expenseResp, historyResp] = await Promise.all([
        expenseDetailApi.getExpense(expenseId),
        expenseDetailApi.getCorrectionHistory(expenseId),
      ]);
      setExpense(expenseResp.expense);
      setHistory(historyResp.entries);

      // Fetch pro-rata group if this is a pro-rata expense
      if (expenseResp.expense.isProRata && expenseResp.expense.proRataGroup) {
        try {
          const groupResp = await expenseDetailApi.getProRataGroup(
            expenseResp.expense.proRataGroup,
          );
          setProRataGroup(groupResp.data);
        } catch {
          // Non-critical: pro-rata group display is supplementary
          setProRataGroup([]);
        }
      } else {
        setProRataGroup([]);
      }

      setStatus("detail");
    } catch {
      setError("Failed to load expense details.");
      setStatus("error");
    }
  }, [expenseId]);

  useEffect(() => {
    fetchExpenseData();
  }, [fetchExpenseData]);

  const optionsRef = useRef(options);
  optionsRef.current = options;

  const correctionMutation = useFormMutation<ExpenseResponse>({
    onSuccess: () => {
      setStatus("detail");
      fetchExpenseData();
      optionsRef.current?.onCorrectionSuccess?.();
    },
  });

  const submitCorrection = useCallback(
    (form: CorrectExpenseRequest) => {
      if (!expense) return;
      correctionMutation.submit(() =>
        expenseDetailApi.submitCorrection(expense.id, form),
      );
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [expense, correctionMutation.submit],
  );

  const refresh = useCallback(() => {
    fetchExpenseData();
  }, [fetchExpenseData]);

  const startCorrection = useCallback(() => {
    setStatus("correct");
  }, []);

  const cancelCorrection = useCallback(() => {
    setStatus("detail");
  }, []);

  const correction: CorrectionState = {
    submitCorrection,
    submitting: correctionMutation.submitting,
    error: correctionMutation.error,
    clearError: correctionMutation.clearError,
  };

  if (status === "loading") {
    return { status: "loading", refresh };
  }

  if (status === "error") {
    return { status: "error", error: error ?? "Unknown error", refresh };
  }

  // At this point, status is "detail" or "correct" and expense must be non-null
  // (we only transition to these statuses after a successful fetch)
  const loadedExpense = expense!;

  if (status === "correct") {
    return {
      status: "correct",
      expense: loadedExpense,
      history,
      proRataGroup,
      correction,
      cancelCorrection,
      refresh,
    };
  }

  return {
    status: "detail",
    expense: loadedExpense,
    history,
    proRataGroup,
    correction,
    startCorrection,
    refresh,
  };
}
