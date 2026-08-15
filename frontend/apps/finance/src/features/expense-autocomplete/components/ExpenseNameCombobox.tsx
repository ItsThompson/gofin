import * as React from "react";

import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@gofin/ui/components/combobox";
import { FormMessage } from "@gofin/ui/components/form";
import { getCurrencySymbol, toMajorUnits, getMinorUnitDigits } from "@gofin/core";

import { useExpenseAutocomplete } from "../hooks/useExpenseAutocomplete";
import type { ExpenseSuggestion } from "../types";

function formatSuggestionAmount(suggestion: ExpenseSuggestion): string {
  const amount = suggestion.transactionAmount ?? suggestion.amount;
  const currency = suggestion.transactionCurrency ?? suggestion.currency;
  const major = toMajorUnits(amount, currency);
  const symbol = getCurrencySymbol(currency);
  const digits = getMinorUnitDigits(currency);
  return `${symbol}${major.toFixed(digits)}`;
}

export interface ExpenseNameComboboxProps {
  id?: string;
  value: string;
  onValueChange: (value: string) => void;
  onSelectSuggestion: (suggestion: ExpenseSuggestion) => void;
  error?: string;
  placeholder?: string;
}

export function ExpenseNameCombobox({
  id = "expense-name",
  value,
  onValueChange,
  onSelectSuggestion,
  error,
  placeholder = "e.g. Grocery shopping",
}: ExpenseNameComboboxProps) {
  const { state, actions } = useExpenseAutocomplete();
  const { loadMore, setQuery } = actions;
  const [isOpen, setIsOpen] = React.useState(false);

  React.useEffect(() => {
    setQuery(value);
  }, [setQuery, value]);

  function handleValueChange(nextValue: string) {
    onValueChange(nextValue);
    setIsOpen(nextValue.trim().length > 0);
  }

  function handleSelectSuggestion(suggestion: ExpenseSuggestion) {
    onValueChange(suggestion.name);
    onSelectSuggestion(suggestion);
  }

  const hasTypedInput = value.trim().length > 0;
  const shouldShowEmpty = hasTypedInput && state.visibleSuggestions.length === 0;
  const loadMoreLabel = state.isLoadingMore ? "Loading more suggestions..." : "Load more suggestions";

  return (
    <Combobox open={hasTypedInput && isOpen} onOpenChange={setIsOpen}>
      <ComboboxInput
        id={id}
        type="text"
        autoComplete="off"
        placeholder={placeholder}
        value={value}
        onChange={(event) => handleValueChange(event.target.value)}
        aria-invalid={!!error}
      />
      <ComboboxContent>
        <ComboboxList>
          {state.visibleSuggestions.map((suggestion) => (
            <ComboboxItem
              key={suggestion.name}
              value={suggestion.name}
              onSelect={() => handleSelectSuggestion(suggestion)}
            >
              <div className="flex flex-col">
                <span>{suggestion.name}</span>
                <span className="text-xs text-muted-foreground">
                  {formatSuggestionAmount(suggestion)} · {suggestion.transactionCurrency ?? suggestion.currency} · Frecency: {suggestion.frecencyScore}
                </span>
              </div>
            </ComboboxItem>
          ))}
          {shouldShowEmpty && <ComboboxEmpty>No matching expenses</ComboboxEmpty>}
          {state.hasMore && (
            <ComboboxItem
              value="load-more-suggestions"
              disabled={state.isLoadingMore}
              closeOnSelect={false}
              onSelect={() => {
                void loadMore();
              }}
              className="text-muted-foreground"
            >
              {loadMoreLabel}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
      <FormMessage>{error}</FormMessage>
    </Combobox>
  );
}
