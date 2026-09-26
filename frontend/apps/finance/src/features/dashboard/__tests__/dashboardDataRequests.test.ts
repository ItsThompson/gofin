import { buildPeriod } from "@gofin/test-utils";
import { describe, expect, it } from "vitest";
import { fetchDashboardSection } from "../hooks/dashboardDataRequests";

const period = buildPeriod({ year: 2026, month: 5 });

describe("fetchDashboardSection", () => {
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
