import { buildPeriod } from "@gofin/test-utils";
import { afterEach, describe, expect, it } from "vitest";
import { fetchDashboardSection } from "../hooks/dashboardDataRequests";

const period = buildPeriod({ year: 2026, month: 5 });

describe("fetchDashboardSection", () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });
  it("passes force refresh to section requests", async () => {
    let cacheControl = "";
    globalThis.fetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
      cacheControl = new Headers(init?.headers).get("Cache-Control") ?? "";
      return Promise.resolve(new Response(JSON.stringify({ summary: {} }), { status: 200 }));
    }) as typeof fetch;

    await fetchDashboardSection("summary", period, 6, new AbortController().signal, true);

    expect(cacheControl).toBe("no-cache");
  });

  it("passes the selected trend window to the trend request", async () => {
    let requestedUrl = "";
    globalThis.fetch = ((input: RequestInfo | URL) => {
      requestedUrl = String(input);
      return Promise.resolve(new Response(JSON.stringify({ trends: [] }), { status: 200 }));
    }) as typeof fetch;

    const result = await fetchDashboardSection("trends", period, 12, new AbortController().signal);

    expect(requestedUrl).toContain("/api/finance/spending/trends?year=2026&month=5&months=12");
    expect(result).toEqual({ section: "trends", data: [] });
  });
});
