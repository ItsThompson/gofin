import { useState, useCallback, useEffect, useRef } from "react";
import { useApiToast, useFormMutation } from "@gofin/api";
import type { Expense, CorrectExpenseRequest, ExpenseResponse, PaginatedResponse } from "@gofin/core";
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

  // The installment list is supplementary to the detail view, but a failure to
  // load it used to remove rows with nothing said. useApiToast owns the report
  // and the message; retry is off because the toast's Retry discards its result.
  const { call: proRataCall } = useApiToast<PaginatedResponse<Expense>>({
    retriable: false,
    op: "expense.pro_rata_group",
    domain: "expenses",
  });

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

      const groupId = expenseResp.expense.proRataGroup;
      if (expenseResp.expense.isProRata && groupId) {
        const groupResp = await proRataCall(() =>
          expenseDetailApi.getProRataGroup(groupId),
        );
        setProRataGroup(groupResp?.data ?? []);
      } else {
        setProRataGroup([]);
      }

      setStatus("detail");
    } catch {
      setError("Failed to load expense details.");
      setStatus("error");
    }
  }, [expenseId, proRataCall]);

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
