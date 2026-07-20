import { describe, it, expect } from "vitest";
import type { PeriodListResponse } from "@gofin/core";
import { periodsHandlers } from "../handlers/periods";
import { resolveMockRequest } from "./drive";

describe("GET /api/finance/periods (dev-mock)", () => {
  it("returns a PeriodListResponse ({ periods }), not a paginated shape", async () => {
    const res = await resolveMockRequest(
      periodsHandlers,
      "/api/finance/periods",
    );
    const body = (await res.json()) as PeriodListResponse & { data?: unknown };

    // The history and expense-log period flows read res.periods; the old mock
    // returned a PaginatedResponse ({ data }), leaving res.periods undefined.
    expect(Array.isArray(body.periods)).toBe(true);
    expect(body.periods.length).toBeGreaterThan(0);
    expect(body.data).toBeUndefined();
  });
});
