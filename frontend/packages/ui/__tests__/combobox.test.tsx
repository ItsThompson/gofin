import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "../src/components/combobox";

function TestCombobox({ onSelect = () => {} }: { onSelect?: (value: string) => void }) {
  return (
    <Combobox>
      <label htmlFor="expense-name">Expense name</label>
      <ComboboxInput id="expense-name" />
      <ComboboxContent>
        <ComboboxEmpty>No matches</ComboboxEmpty>
        <ComboboxList>
          <ComboboxItem value="coffee" onSelect={onSelect}>
            <span>Coffee</span>
            <span>Frecency score: 12</span>
          </ComboboxItem>
          <ComboboxItem value="rent" onSelect={onSelect}>
            <span>Rent</span>
            <span>Frecency score: 9</span>
          </ComboboxItem>
          <ComboboxItem value="disabled" disabled onSelect={onSelect}>
            Disabled option
          </ComboboxItem>
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}

describe("Combobox", () => {
  it("exposes combobox, listbox, and option roles with custom item content", async () => {
    const user = userEvent.setup();
    render(<TestCombobox />);

    const input = screen.getByRole("combobox", { name: "Expense name" });
    await user.click(input);

    expect(input).toHaveAttribute("aria-expanded", "true");
    expect(input).toHaveAttribute("aria-controls", screen.getByRole("listbox").id);
    expect(screen.getByRole("option", { name: /Coffee.*Frecency score: 12/ })).toBeInTheDocument();
    expect(screen.getByText("No matches")).toBeInTheDocument();
  });

  it("moves highlight with ArrowDown and ArrowUp without selecting", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<TestCombobox onSelect={onSelect} />);

    const input = screen.getByRole("combobox", { name: "Expense name" });
    await user.click(input);
    await user.keyboard("{ArrowDown}");

    expect(screen.getByRole("option", { name: /Coffee/ })).toHaveAttribute("aria-selected", "true");
    expect(onSelect).not.toHaveBeenCalled();

    await user.keyboard("{ArrowDown}{ArrowUp}");

    expect(screen.getByRole("option", { name: /Coffee/ })).toHaveAttribute("aria-selected", "true");
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("selects highlighted item on Enter", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<TestCombobox onSelect={onSelect} />);

    await user.click(screen.getByRole("combobox", { name: "Expense name" }));
    await user.keyboard("{ArrowDown}{Enter}");

    expect(onSelect).toHaveBeenCalledWith("coffee");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("does not select on Enter when no item is highlighted", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<TestCombobox onSelect={onSelect} />);

    await user.click(screen.getByRole("combobox", { name: "Expense name" }));
    await user.keyboard("{Enter}");

    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.getByRole("listbox")).toBeInTheDocument();
  });

  it("closes on Escape without selecting", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<TestCombobox onSelect={onSelect} />);

    await user.click(screen.getByRole("combobox", { name: "Expense name" }));
    await user.keyboard("{ArrowDown}{Escape}");

    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("closes on blur without selecting", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <div>
        <TestCombobox onSelect={onSelect} />
        <button>Next field</button>
      </div>,
    );

    await user.click(screen.getByRole("combobox", { name: "Expense name" }));
    await user.keyboard("{ArrowDown}");
    await user.click(screen.getByRole("button", { name: "Next field" }));

    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });
});
