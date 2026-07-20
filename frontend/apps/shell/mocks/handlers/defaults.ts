import { http, HttpResponse } from "msw";
import type {
  DefaultSettings,
  DefaultsResponse,
  UpdateDefaultsRequest,
} from "@gofin/core";
import { mockDefaults } from "../data";
import { simulateLatency } from "./latency";

export const defaultsHandlers = [
  http.get<never, never, DefaultsResponse>("/api/finance/defaults", async () => {
    await simulateLatency();
    return HttpResponse.json({ defaults: mockDefaults });
  }),

  http.put<never, UpdateDefaultsRequest, DefaultsResponse>(
    "/api/finance/defaults",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const defaults: DefaultSettings = {
        ...mockDefaults,
        ...body,
        updatedAt: new Date().toISOString(),
      };
      return HttpResponse.json({ defaults });
    },
  ),

  http.post<never, UpdateDefaultsRequest, DefaultsResponse>(
    "/api/finance/onboarding",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const defaults: DefaultSettings = {
        ...mockDefaults,
        ...body,
        updatedAt: new Date().toISOString(),
      };
      return HttpResponse.json({ defaults });
    },
  ),
];
