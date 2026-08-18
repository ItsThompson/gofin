import { useState, useCallback, useEffect, useRef, type FormEvent } from "react";
import { useBudgetSplitForm, useFormMutation } from "@gofin/api";
import type { User } from "@gofin/core";
import type { UpdateDefaultsRequest } from "@gofin/core";
import { settingsApi } from "../api";
import type { SaveStatus } from "../types";

export interface DefaultBudgetState {
  budgetDollars: string;
  essentials: string;
  desires: string;
  savings: string;
  currency: string;
  /** Live validation error from the split fields, or null when valid. */
  validationError: string | null;
  /** Single status for the save operation; failure message travels with `failed`. */
  saveStatus: SaveStatus;
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
  const [currency, setCurrency] = useState(user.currency);
  const form = useBudgetSplitForm({ currency });
  const [saveStatus, setSaveStatus] = useState<SaveStatus>({ kind: "idle" });
  const [fetching, setFetching] = useState(true);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const { submit } = useFormMutation<void>({
    onSuccess: () => {
      setSaveStatus({ kind: "saved" });
      timeoutRef.current = setTimeout(
        () => setSaveStatus({ kind: "idle" }),
        3000,
      );
    },
    onError: (message) => {
      setSaveStatus({ kind: "failed", message });
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
          currency: defaults.currency,
          initialSplit: {
            essentials: defaults.essentialsPercent,
            desires: defaults.desiresPercent,
            savings: defaults.savingsPercent,
          },
        });
        setCurrency(defaults.currency);
      } catch {
        // Use fallback defaults (hook already uses DEFAULT_BUDGET_SPLIT)
        form.reset({ initialBudgetCents: 0, currency });
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
      setSaveStatus({ kind: "idle" });

      if (form.validate()) {
        return;
      }

      setSaveStatus({ kind: "saving" });

      const payload = form.toPayload();

      const body: UpdateDefaultsRequest = {
        budgetAmount: payload.budgetAmountCents,
        essentialsPercent: payload.essentialsPercent,
        desiresPercent: payload.desiresPercent,
        savingsPercent: payload.savingsPercent,
        currency,
      };

      submit(async () => {
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
    [form, currency, submit],
  );

  return {
    state: {
      budgetDollars: form.fields.budgetDollars,
      essentials: form.fields.essentials,
      desires: form.fields.desires,
      savings: form.fields.savings,
      currency,
      validationError: form.splitError,
      saveStatus,
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
