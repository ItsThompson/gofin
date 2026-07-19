import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { HeroAnimation } from "../components/HeroAnimation";

const ALT = "A GoFin spending breakdown split into essentials, desires, and savings.";

/** Drive the hero's reduced-motion detection by faking matchMedia.matches. */
function setPrefersReducedMotion(reduce: boolean): void {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: reduce && query.includes("prefers-reduced-motion"),
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

beforeEach(() => setPrefersReducedMotion(false));
afterEach(() => setPrefersReducedMotion(false));

describe("HeroAnimation", () => {
  it("mounts an animated scene labelled for assistive tech", () => {
    render(<HeroAnimation alt={ALT} />);

    expect(screen.getByRole("img", { name: ALT })).toBeInTheDocument();
  });

  it("renders the log-expense scene and the dashboard on the animated path", () => {
    render(<HeroAnimation alt={ALT} />);

    // LogExpenseCard is present only on the animated (non-reduced) path.
    expect(screen.getByText("Log expense")).toBeInTheDocument();
    expect(screen.getByText("Groceries")).toBeInTheDocument();
    expect(screen.getByText("Log Expense")).toBeInTheDocument();
    // DashboardPreviewCard is stacked underneath.
    expect(screen.getByText("This month")).toBeInTheDocument();
  });

  it("renders only the static dashboard end state under reduced motion", () => {
    setPrefersReducedMotion(true);

    render(<HeroAnimation alt={ALT} />);

    // No log-expense scene, no loop: just the settled dashboard.
    expect(screen.queryByText("Log expense")).not.toBeInTheDocument();
    expect(screen.queryByText("Log Expense")).not.toBeInTheDocument();

    expect(screen.getByText("This month")).toBeInTheDocument();
    expect(screen.getByText("Expense logged")).toBeInTheDocument();
    for (const label of ["Essentials", "Desires", "Savings"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });
});
