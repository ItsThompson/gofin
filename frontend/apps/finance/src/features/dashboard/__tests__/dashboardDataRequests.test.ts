import { afterEach, describe, expect, it } from "vitest";
import { fetchDashboardSection } from "../hooks/dashboardDataRequests";

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("fetchDashboardSection", () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("passes force refresh to paginated suggestions", async () => {
    let cacheControl = "";
    globalThis.fetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
      cacheControl = new Headers(init?.headers).get("Cache-Control") ?? "";
      return Promise.resolve(response({ data: [], hasMore: false }));
    }) as typeof fetch;

    const result = await fetchDashboardSection(
      new AbortController().signal,
      true,
    );

    expect(cacheControl).toBe("no-cache");
    expect(result).toEqual({ section: "suggestions", data: [] });
  });

  it("fetches later pages when earlier pages contain no active suggestions", async () => {
    const requestedUrls: string[] = [];
    globalThis.fetch = ((input: RequestInfo | URL) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.includes("page=1"))
        return Promise.resolve(
          response({
            data: [{ name: "Old", recencyBucket: "older" }],
            hasMore: true,
          }),
        );
      return Promise.resolve(
        response({
          data: [
            {
              name: "Coffee",
              active: true,
              originalTransactionAmountInMinorUnits: 100,
              transactionCurrencyCode: "USD",
              expenseType: "desires",
              frequency: 1,
              lastUsedAt: "2026-05-01",
              recencyBucket: "today",
              frecencyScore: 1,
            },
          ],
          hasMore: false,
        }),
      );
    }) as typeof fetch;

    const result = await fetchDashboardSection(new AbortController().signal);

    expect(requestedUrls).toHaveLength(2);
    expect(result.data).toHaveLength(1);
    expect(result.data[0]?.name).toBe("Coffee");
  });
});
