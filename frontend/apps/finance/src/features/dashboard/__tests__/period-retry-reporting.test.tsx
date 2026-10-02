import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMockApi } from "@gofin/test-utils";
import { renderDashboard } from "./render";
import { renderWithRouter } from "@gofin/test-utils";
import { ActiveDashboard } from "../components/ActiveDashboard";
import { dashboardDataEmptyRoutes, testPeriod } from "./fixtures";

interface CapturedContext { tags?: Record<string, string> }
const { captureException } = vi.hoisted(() => ({ captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(() => "event-id") }));
vi.mock("@sentry/react-router", () => ({ captureException }));

beforeEach(() => {
  captureException.mockClear();
  window.matchMedia = vi.fn().mockImplementation(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }));
});

describe("dashboard period retry reporting", () => {
  it("reports a failed period lookup once through the dashboard retry UI", async () => {
    globalThis.fetch = createMockApi({
      "/api/finance/periods/current": { status: 503, body: { code: "UNAVAILABLE", message: "Period lookup failed" } },
      ...dashboardDataEmptyRoutes(),
      "/api/finance/health-score/trend": { body: { trends: [] } },
      "/api/finance/health-score": { body: { healthScore: { configureBudget: true } } },
    }) as unknown as typeof fetch;
    const user = userEvent.setup();
    renderWithRouter(<ActiveDashboard period={testPeriod} />);

    await waitFor(() => expect(screen.getByText("No expenses yet")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "Refresh all data" }));
    await waitFor(() => expect(screen.getByText("Could not load the dashboard")).toBeInTheDocument());
    expect(captureException).toHaveBeenCalledTimes(1);
    expect(captureException.mock.calls[0][1]?.tags).toMatchObject({ operation: "budget.period", domain: "budgets" });
  });

  it("verifies period metadata before retrying summary without reloading completed sections", async () => {
    let summaryCalls = 0;
    let recentExpenseCalls = 0;
    let healthScoreCalls = 0;
    let periodVerificationCalls = 0;
    const mockApi = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      ...dashboardDataEmptyRoutes(),
      "/api/finance/health-score/trend": { body: { trends: [] } },
      "/api/finance/health-score": { body: { healthScore: { configureBudget: true } } },
    }) as unknown as typeof fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/finance/summary")) {
        summaryCalls += 1;
        if (summaryCalls === 1) return Promise.resolve(new Response(JSON.stringify({ code: "PERIOD_NOT_FOUND" }), { status: 404 }));
      }
      if (url.includes("/api/expenses?")) recentExpenseCalls += 1;
      if (url.includes("/api/finance/health-score") && !url.includes("/trend")) healthScoreCalls += 1;
      if (url.includes("/api/finance/periods/current")) periodVerificationCalls += 1;
      return mockApi(input, init);
    }) as typeof fetch;

    renderWithRouter(<ActiveDashboard period={testPeriod} />);
    await waitFor(() => expect(screen.getByText("No expenses yet")).toBeInTheDocument());
    await waitFor(() => expect(summaryCalls).toBe(2));

    expect(periodVerificationCalls).toBe(1);
    expect(recentExpenseCalls).toBe(1);
    expect(healthScoreCalls).toBe(1);
  });

  it("reports section failures while keeping unrelated dashboard content visible", async () => {
    globalThis.fetch = createMockApi({
      "/api/finance/periods/current": { body: { period: testPeriod } },
      ...dashboardDataEmptyRoutes(),
      "/api/finance/health-score": { status: 503, body: { code: "UPSTREAM_UNAVAILABLE", message: "Health score failed" } },
    }) as unknown as typeof fetch;
    renderDashboard();

    await waitFor(() => expect(screen.getByText("Could not load Health Score")).toBeInTheDocument());
    expect(screen.getByText("No expenses yet")).toBeInTheDocument();
    expect(captureException.mock.calls.some(([, context]) => context?.tags?.operation === "dashboard.healthScore")).toBe(true);
  });

  it("ignores a late period failure after the dashboard unmounts", async () => {
    let rejectPeriod!: (error: Error) => void;
    const mockApi = createMockApi({ "/api/finance/periods/current": { body: { period: testPeriod } }, ...dashboardDataEmptyRoutes() });
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes("/api/finance/periods/current") && init?.headers) {
        return new Promise<Response>((_resolve, reject) => { rejectPeriod = reject; });
      }
      return mockApi(input, init);
    }) as typeof fetch;
    const { unmount } = renderWithRouter(<ActiveDashboard period={testPeriod} />);
    await waitFor(() => expect(screen.getByText("No expenses yet")).toBeInTheDocument());
    captureException.mockClear();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Refresh all data" }));
    unmount();
    await act(async () => rejectPeriod(new Error("Late period failure")));
    expect(captureException).not.toHaveBeenCalled();
  });
});
