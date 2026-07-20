import { http, HttpResponse } from "msw";
import type { User, ApiError } from "@gofin/core";
import { currentMockUser, allUsers } from "../data";
import { simulateLatency } from "./latency";

export const authHandlers = [
  http.get<never, never, { user: User }>("/api/auth/me", async () => {
    await simulateLatency();
    return HttpResponse.json({ user: currentMockUser });
  }),

  http.post<never, { email: string; password: string }, { user: User } | ApiError>(
    "/api/auth/login",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const user = allUsers.find((u) => u.email === body.email);
      if (!user) {
        return HttpResponse.json(
          { code: "INVALID_CREDENTIALS", message: "Invalid email or password" },
          { status: 401 },
        );
      }
      return HttpResponse.json({ user });
    },
  ),

  http.post<never, { username: string; email: string }, { user: User }>(
    "/api/auth/register",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const newUser: User = {
        ...currentMockUser,
        id: crypto.randomUUID(),
        username: body.username,
        email: body.email,
        hasCompletedOnboarding: false,
      };
      return HttpResponse.json({ user: newUser });
    },
  ),

  http.post("/api/auth/logout", async () => {
    await simulateLatency();
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("/api/auth/refresh", async () => {
    await simulateLatency();
    return new HttpResponse(null, { status: 204 });
  }),

  http.put<never, { username: string; email: string; currency: string }, { user: User }>(
    "/api/auth/me",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const updated: User = { ...currentMockUser, ...body };
      return HttpResponse.json({ user: updated });
    },
  ),

  http.post("/api/auth/me/password", async () => {
    await simulateLatency();
    return new HttpResponse(null, { status: 204 });
  }),

  http.post<never, never, { user: User }>("/api/auth/onboarding-complete", async () => {
    await simulateLatency();
    const updated: User = { ...currentMockUser, hasCompletedOnboarding: true };
    return HttpResponse.json({ user: updated });
  }),

  http.post<never, { userId: string }, { user: User } | ApiError>(
    "/api/auth/assume",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const target = allUsers.find((u) => u.id === body.userId);
      if (!target) {
        return HttpResponse.json(
          { code: "NOT_FOUND", message: "User not found" },
          { status: 404 },
        );
      }
      return HttpResponse.json({ user: target });
    },
  ),

  http.post<never, never, { user: User }>("/api/auth/restore", async () => {
    await simulateLatency();
    return HttpResponse.json({ user: currentMockUser });
  }),
];
