import { useState, useCallback, useEffect, type FormEvent } from "react";
import { ApiRequestError } from "@gofin/api";
import type { User } from "@gofin/core";
import type { UpdateDefaultsRequest } from "@/types";
import { settingsApi } from "../api";

/**
 * Validates that E/D/S percentages sum to exactly 100.
 * Returns an error message or null if valid.
 */
function validateEDSSplit(essentials: number, desires: number, savings: number): string | null {
  const total = essentials + desires + savings;
  if (total !== 100) {
    return `Percentages must sum to 100% (currently ${total}%)`;
  }
  return null;
}

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
  const [budgetDollars, setBudgetDollars] = useState("");
  const [essentials, setEssentials] = useState("");
  const [desires, setDesires] = useState("");
  const [savings, setSavings] = useState("");
  const [currency, setCurrency] = useState(user.currency);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [loading, setLoading] = useState(false);
  const [fetching, setFetching] = useState(true);

  useEffect(() => {
    async function fetchDefaults() {
      try {
        const response = await settingsApi.getDefaults();
        const defaults = response.defaults;
        setBudgetDollars(String(defaults.budgetAmount / 100));
        setEssentials(String(defaults.essentialsPercent));
        setDesires(String(defaults.desiresPercent));
        setSavings(String(defaults.savingsPercent));
        setCurrency(defaults.currency);
      } catch {
        // Use fallback defaults
        setBudgetDollars("0");
        setEssentials("50");
        setDesires("30");
        setSavings("20");
      } finally {
        setFetching(false);
      }
    }
    fetchDefaults();
  }, []);

  const handleSubmit = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      setError(null);
      setSuccess(false);

      const essentialsNum = parseInt(essentials, 10) || 0;
      const desiresNum = parseInt(desires, 10) || 0;
      const savingsNum = parseInt(savings, 10) || 0;

      const splitError = validateEDSSplit(essentialsNum, desiresNum, savingsNum);
      if (splitError) {
        setError(splitError);
        return;
      }

      const budgetCents = Math.round((parseFloat(budgetDollars) || 0) * 100);

      setLoading(true);

      try {
        const body: UpdateDefaultsRequest = {
          budgetAmount: budgetCents,
          essentialsPercent: essentialsNum,
          desiresPercent: desiresNum,
          savingsPercent: savingsNum,
          currency,
        };

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

        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        if (err instanceof ApiRequestError) {
          setError(err.message);
        } else {
          setError("An unexpected error occurred. Please try again.");
        }
      } finally {
        setLoading(false);
      }
    },
    [budgetDollars, essentials, desires, savings, currency],
  );

  return {
    state: { budgetDollars, essentials, desires, savings, currency, error, success, loading, fetching },
    actions: { setBudgetDollars, setEssentials, setDesires, setSavings, setCurrency, handleSubmit },
  };
}
