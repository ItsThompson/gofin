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

import { useExpenseAutocomplete } from "../hooks/useExpenseAutocomplete";
import type { ExpenseSuggestion } from "../types";

export interface ExpenseNameComboboxProps {
  value: string;
  onValueChange: (value: string) => void;
  onSelectSuggestion: (suggestion: ExpenseSuggestion) => void;
  error?: string;
}

export function ExpenseNameCombobox({
  value,
  onValueChange,
  onSelectSuggestion,
  error,
}: ExpenseNameComboboxProps) {
  const { state, actions } = useExpenseAutocomplete();
  const { setQuery } = actions;

  React.useEffect(() => {
    setQuery(value);
  }, [setQuery, value]);

  function handleValueChange(nextValue: string) {
    onValueChange(nextValue);
    setQuery(nextValue);
  }

  function handleSelectSuggestion(suggestion: ExpenseSuggestion) {
    onValueChange(suggestion.name);
    setQuery(suggestion.name);
    onSelectSuggestion(suggestion);
  }

  const hasTypedInput = value.trim().length > 0;
  const shouldShowEmpty = hasTypedInput && state.visibleSuggestions.length === 0;

  return (
    <Combobox>
      <ComboboxInput
        id="expense-name"
        type="text"
        placeholder="e.g. Grocery shopping"
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
                  Frecency score: {suggestion.frecencyScore}
                </span>
              </div>
            </ComboboxItem>
          ))}
          {shouldShowEmpty && <ComboboxEmpty>No matching expenses</ComboboxEmpty>}
        </ComboboxList>
      </ComboboxContent>
      <FormMessage>{error}</FormMessage>
    </Combobox>
  );
}
