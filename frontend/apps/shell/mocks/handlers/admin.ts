import { http, HttpResponse } from "msw";
import type { AdminUsersResponse, ApiError } from "@gofin/core";
import { allUsers } from "../data";
import { simulateLatency } from "./latency";

export const adminHandlers = [
  http.get<never, never, AdminUsersResponse>("/api/admin/users", async () => {
    await simulateLatency();
    return HttpResponse.json({ users: allUsers });
  }),

  http.delete<{ id: string }, { password: string }, ApiError>(
    "/api/admin/users/:id",
    async ({ request, params }) => {
      await simulateLatency();
      const body = await request.json();
      const targetId = params.id;

      // Simulate password check: accept any non-empty password in dev-mock
      if (!body.password) {
        return HttpResponse.json(
          { code: "VALIDATION_ERROR", message: "Invalid request body" },
          { status: 400 },
        );
      }

      // Simulate wrong password with a known test value
      if (body.password === "wrong") {
        return HttpResponse.json(
          { code: "INVALID_CREDENTIALS", message: "Invalid password" },
          { status: 401 },
        );
      }

      const target = allUsers.find((u) => u.id === targetId);
      if (!target) {
        return HttpResponse.json(
          { code: "NOT_FOUND", message: "User not found" },
          { status: 404 },
        );
      }

      // Simulate protected user check
      if (target.username === "admin" || target.username === "thompson") {
        return HttpResponse.json(
          { code: "PROTECTED_USER", message: "Cannot delete a protected user" },
          { status: 403 },
        );
      }

      return new HttpResponse(null, { status: 204 });
    },
  ),
];
