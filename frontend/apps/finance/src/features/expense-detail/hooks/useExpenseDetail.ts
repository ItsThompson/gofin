import { useState, useCallback, useEffect, useRef } from "react";
import { useFormMutation } from "@gofin/api";
import type { Expense, CorrectExpenseRequest } from "@/types";
import { expenseDetailApi } from "../api";

export type DetailViewState = "loading" | "detail" | "correct" | "error";

export interface UseExpenseDetailOptions {
  /** Called after a correction is successfully submitted. */
  onCorrectionSuccess?: () => void;
}

export interface ExpenseDetailResult {
  expense: Expense | null;
  history: Expense[];
  proRataGroup: Expense[];
  viewState: DetailViewState;
  error: string | null;
  setViewState: (state: DetailViewState) => void;
  submitCorrection: (form: CorrectExpenseRequest) => void;
  correctionSubmitting: boolean;
  correctionError: string | null;
  clearCorrectionError: () => void;
  refresh: () => void;
}

export function useExpenseDetail(
  expenseId: string | null,
  options?: UseExpenseDetailOptions,
): ExpenseDetailResult {
  const [expense, setExpense] = useState<Expense | null>(null);
  const [history, setHistory] = useState<Expense[]>([]);
  const [proRataGroup, setProRataGroup] = useState<Expense[]>([]);
  const [viewState, setViewState] = useState<DetailViewState>("loading");
  const [error, setError] = useState<string | null>(null);

  const fetchExpenseData = useCallback(async () => {
    if (!expenseId) return;
    setViewState("loading");
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

      setViewState("detail");
    } catch {
      setError("Failed to load expense details.");
      setViewState("error");
    }
  }, [expenseId]);

  useEffect(() => {
    fetchExpenseData();
  }, [fetchExpenseData]);

  const optionsRef = useRef(options);
  optionsRef.current = options;

  const correctionMutation = useFormMutation<void>({
    onSuccess: () => {
      setViewState("detail");
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
    [expense, correctionMutation],
  );

  const refresh = useCallback(() => {
    fetchExpenseData();
  }, [fetchExpenseData]);

  return {
    expense,
    history,
    proRataGroup,
    viewState,
    error,
    setViewState,
    submitCorrection,
    correctionSubmitting: correctionMutation.submitting,
    correctionError: correctionMutation.error,
    clearCorrectionError: correctionMutation.clearError,
    refresh,
  };
}
