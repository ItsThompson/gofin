import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildPeriod, buildPeriodSummary } from "@gofin/test-utils";
import { MemoryRouter } from "react-router";
import { BreakdownDashboardSection } from "../components/sections/BreakdownDashboardSection";
import { HealthScoreDashboardSection } from "../components/sections/HealthScoreDashboardSection";
import { RecentExpensesDashboardSection } from "../components/sections/RecentExpensesDashboardSection";
import { SummaryDashboardSection } from "../components/sections/SummaryDashboardSection";

const period = buildPeriod({ id: "period-1", year: 2026, month: 5 });
const props = { period, enabled: true, refreshVersion: 0, currency: "USD" };

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  vi.restoreAllMocks();
  window.matchMedia = vi.fn().mockImplementation(() => ({
    matches: true,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
});

describe("dashboard section consumers", () => {
  it("owns summary failures and exposes its retry UI", async () => {
    let summaryCalls = 0;
    globalThis.fetch = ((input: RequestInfo | URL) => {
      if (String(input).includes("/summary")) {
        summaryCalls += 1;
        return Promise.resolve(
          response(
            { code: "UPSTREAM_UNAVAILABLE", message: "Summary failed" },
            503,
          ),
        );
      }
      return Promise.resolve(response({}));
    }) as typeof fetch;
    render(
      <SummaryDashboardSection
        {...props}
        summaryRetryVersion={0}
        onPeriodNotFound={() => false}
        onSuccess={() => undefined}
        onRetry={() => undefined}
      />,
    );

    expect(
      await screen.findByText("Could not load Summary"),
    ).toBeInTheDocument();
    expect(summaryCalls).toBe(1);
  });

  it("does not bypass the cache on the initial section load", async () => {
    let cacheControl = "";
    globalThis.fetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
      cacheControl = new Headers(init?.headers).get("Cache-Control") ?? "";
      return Promise.resolve(
        response({
          summary: buildPeriodSummary({
            periodId: period.id,
            year: period.year,
            month: period.month,
          }),
        }),
      );
    }) as typeof fetch;

    render(
      <SummaryDashboardSection
        {...props}
        summaryRetryVersion={0}
        onPeriodNotFound={() => false}
        onSuccess={() => undefined}
        onRetry={() => undefined}
      />,
    );
    await waitFor(() => expect(cacheControl).toBe(""));
  });

  it("keeps health and recent expense consumers independent", async () => {
    globalThis.fetch = ((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/health-score/trend"))
        return Promise.resolve(response({ trends: [] }));
      if (url.includes("/health-score"))
        return Promise.resolve(
          response({ healthScore: { configureBudget: true } }),
        );
      if (url.includes("/expenses?"))
        return Promise.resolve(response({ data: [], hasMore: false }));
      return Promise.resolve(response({}));
    }) as typeof fetch;
    render(
      <MemoryRouter>
        <>
          <HealthScoreDashboardSection {...props} />
          <RecentExpensesDashboardSection {...props} readOnly />
        </>
      </MemoryRouter>,
    );

    await waitFor(() =>
      expect(
        screen.getByText("No expenses recorded for this period."),
      ).toBeInTheDocument(),
    );
    expect(
      screen.getByText("No health score trend is available."),
    ).toBeInTheDocument();
  });

  it("keeps no-cache when a deferred section is retried before resuming", async () => {
    let byTagCalls = 0;
    let resumedCacheControl = "";
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/spending/by-tag")) {
        byTagCalls += 1;
        if (byTagCalls === 1)
          return Promise.resolve(response({ code: "UNAVAILABLE" }, 503));
        resumedCacheControl =
          new Headers(init?.headers).get("Cache-Control") ?? "";
        return Promise.resolve(response({ tagSpending: [] }));
      }
      return Promise.resolve(response({}));
    }) as typeof fetch;
    const user = userEvent.setup();
    const view = render(
      <BreakdownDashboardSection
        {...props}
        desktopVisible
        selectedChart="tag-spending"
        onSelectedChartChange={() => undefined}
      />,
    );

    await screen.findByText("Could not load Tag spending");
    view.rerender(
      <BreakdownDashboardSection
        {...props}
        enabled={false}
        desktopVisible
        selectedChart="tag-spending"
        onSelectedChartChange={() => undefined}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Retry" }));
    view.rerender(
      <BreakdownDashboardSection
        {...props}
        enabled
        desktopVisible
        selectedChart="tag-spending"
        onSelectedChartChange={() => undefined}
      />,
    );

    await waitFor(() => expect(byTagCalls).toBe(2));
    expect(resumedCacheControl).toBe("no-cache");
  });

  it("loads repeated-expense suggestions only after the breakdown selection", async () => {
    const requestedUrls: string[] = [];
    globalThis.fetch = ((input: RequestInfo | URL) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.includes("/spending/by-tag"))
        return Promise.resolve(response({ tagSpending: [] }));
      if (url.includes("/expenses/suggestions"))
        return Promise.resolve(response({ data: [], hasMore: false }));
      return Promise.resolve(response({}));
    }) as typeof fetch;
    render(
      <BreakdownDashboardSection
        {...props}
        desktopVisible
        selectedChart="tag-spending"
        onSelectedChartChange={() => undefined}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getByLabelText("Select breakdown chart"),
      ).toBeInTheDocument(),
    );
    expect(
      requestedUrls.some((url) => url.includes("/expenses/suggestions")),
    ).toBe(false);
  });
});
