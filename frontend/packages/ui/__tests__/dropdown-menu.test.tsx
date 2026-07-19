import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "../src/components/dropdown-menu";

function TestMenu({ onSelect = () => {} }: { onSelect?: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger>Open menu</DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem onSelect={onSelect}>First item</DropdownMenuItem>
        <DropdownMenuItem>Second item</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

describe("DropdownMenu", () => {
  it("stays closed until the trigger is clicked", () => {
    render(<TestMenu />);

    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem")).not.toBeInTheDocument();
  });

  it("opens on click and exposes focusable menu items", async () => {
    const user = userEvent.setup();
    render(<TestMenu />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(await screen.findByRole("menu")).toBeInTheDocument();
    const items = screen.getAllByRole("menuitem");
    expect(items).toHaveLength(2);

    // radix moves focus into the menu and highlights the first item on open.
    await user.keyboard("{ArrowDown}");
    expect(items[0]).toHaveFocus();
  });

  it("invokes the item handler when an item is selected", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<TestMenu onSelect={onSelect} />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));
    await user.click(await screen.findByRole("menuitem", { name: "First item" }));

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("closes on Escape", async () => {
    const user = userEvent.setup();
    render(<TestMenu />);

    await user.click(screen.getByRole("button", { name: "Open menu" }));
    expect(await screen.findByRole("menu")).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
});
