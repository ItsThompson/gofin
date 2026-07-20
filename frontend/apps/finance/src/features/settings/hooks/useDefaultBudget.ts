import { useState, useCallback, useEffect, useRef, type FormEvent } from "react";
import { useBudgetSplitForm, useFormMutation } from "@gofin/api";
import type { User } from "@gofin/core";
import type { UpdateDefaultsRequest } from "@gofin/core";
import { settingsApi } from "../api";

export interface DefaultBudgetState {
  budgetDollars: string;
  essentials: string;
  desires: string;
  savings: string;
  currency: string;
  error: string | null;
  success: boolean;
  loading: boolean;
  fetching: boolean;
}

export interface DefaultBudgetActions {
  setBudgetDollars: (value: string) => void;
  setEssentials: (value: string) => void;
  setDesires: (value: string) => void;
  setSavings: (value: string) => void;
  setCurrency: (value: string) => void;
  handleSubmit: (event: FormEvent) => void;
}

export function useDefaultBudget(user: User): { state: DefaultBudgetState; actions: DefaultBudgetActions } {
  const form = useBudgetSplitForm();
  const [currency, setCurrency] = useState(user.currency);
  const [success, setSuccess] = useState(false);
  const [fetching, setFetching] = useState(true);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const mutation = useFormMutation<void>({
    onSuccess: () => {
      setSuccess(true);
      const timeout = setTimeout(() => setSuccess(false), 3000);
      timeoutRef.current = timeout;
    },
  });

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    async function fetchDefaults() {
      try {
        const response = await settingsApi.getDefaults();
        const defaults = response.defaults;
        form.reset({
          initialBudgetCents: defaults.budgetAmount,
          initialSplit: {
            essentials: defaults.essentialsPercent,
            desires: defaults.desiresPercent,
            savings: defaults.savingsPercent,
          },
        });
        setCurrency(defaults.currency);
      } catch {
        // Use fallback defaults (hook already uses DEFAULT_BUDGET_SPLIT)
        form.reset({ initialBudgetCents: 0 });
      } finally {
        setFetching(false);
      }
    }
    fetchDefaults();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSubmit = useCallback(
    (event: FormEvent) => {
      event.preventDefault();
      setSuccess(false);

      const validationError = form.validate();
      if (validationError) {
        mutation.clearError();
        // Validation errors are displayed via the form's split error;
        // surface them through mutation.error for consistency.
        return;
      }

      const payload = form.toPayload();

      const body: UpdateDefaultsRequest = {
        budgetAmount: payload.budgetAmountCents,
        essentialsPercent: payload.essentialsPercent,
        desiresPercent: payload.desiresPercent,
        savingsPercent: payload.savingsPercent,
        currency,
      };

      mutation.submit(async () => {
        await settingsApi.updateDefaults(body);

        // Sync currency to auth service. Fetch the current profile
        // from the server to avoid sending stale username/email if the
        // user edited their profile in another tab before saving here.
        const currentProfile = await settingsApi.getProfile();
        await settingsApi.updateProfile({
          username: currentProfile.user.username,
          email: currentProfile.user.email,
          currency,
        });
      });
    },
    [form, currency, mutation],
  );

  return {
    state: {
      budgetDollars: form.fields.budgetDollars,
      essentials: form.fields.essentials,
      desires: form.fields.desires,
      savings: form.fields.savings,
      currency,
      error: form.splitError || mutation.error,
      success,
      loading: mutation.submitting,
      fetching,
    },
    actions: {
      setBudgetDollars: (value: string) => form.setField("budgetDollars", value),
      setEssentials: (value: string) => form.setField("essentials", value),
      setDesires: (value: string) => form.setField("desires", value),
      setSavings: (value: string) => form.setField("savings", value),
      setCurrency,
      handleSubmit,
    },
  };
}
