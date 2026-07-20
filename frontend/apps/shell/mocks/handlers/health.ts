import { http, HttpResponse } from "msw";

export const healthHandlers = [
  http.get<never, never, { status: string }>("/api/health", () => {
    return HttpResponse.json({ status: "ok" });
  }),
];
