import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DashboardOutline } from "../components/DashboardOutline";

describe("DashboardOutline", () => {
  it("renders a nav with Dashboard sections aria-label", () => {
    render(<DashboardOutline />);
    expect(screen.getByRole("navigation", { name: "Dashboard sections" })).toBeInTheDocument();
  });

  it("has responsive visibility classes (hidden on small, visible on xl)", () => {
    const { container } = render(<DashboardOutline />);
    const nav = container.querySelector("nav");
    expect(nav?.className).toContain("hidden");
    expect(nav?.className).toContain("xl:block");
  });

  it("has fixed positioning classes", () => {
    const { container } = render(<DashboardOutline />);
    const nav = container.querySelector("nav");
    expect(nav?.className).toContain("fixed");
  });

  it("renders all top-level section links", () => {
    render(<DashboardOutline />);
    const links = screen.getAllByRole("link");
    const linkTexts = links.map((link) => link.textContent);

    expect(linkTexts).toContain("Summary");
    expect(linkTexts).toContain("Trends");
    expect(linkTexts).toContain("Breakdown");
    expect(linkTexts).toContain("Cumulative Spending");
    expect(linkTexts).toContain("Recent Expenses");
  });

  it("renders nested child links under Summary", () => {
    render(<DashboardOutline />);
    const links = screen.getAllByRole("link");
    const linkTexts = links.map((link) => link.textContent);

    expect(linkTexts).toContain("Budget Allocations");
    expect(linkTexts).toContain("Spending Pace");
    expect(linkTexts).toContain("Historical Comparison");
  });

  it("links have correct href attributes for anchor navigation", () => {
    render(<DashboardOutline />);

    expect(screen.getByRole("link", { name: "Summary" })).toHaveAttribute("href", "#summary");
    expect(screen.getByRole("link", { name: "Spending Pace" })).toHaveAttribute("href", "#spending-pace");
    expect(screen.getByRole("link", { name: "Cumulative Spending" })).toHaveAttribute("href", "#cumulative-spending");
    expect(screen.getByRole("link", { name: "Recent Expenses" })).toHaveAttribute("href", "#recent-expenses");
  });
});
