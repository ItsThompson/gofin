import { http, HttpResponse } from "msw";
import type { Tag, TagListResponse, TagResponse, ApiError } from "@gofin/core";
import { mockTags } from "../data";
import { simulateLatency } from "./latency";

export const tagsHandlers = [
  http.get<never, never, TagListResponse>("/api/finance/tags", async () => {
    await simulateLatency();
    return HttpResponse.json({ tags: mockTags });
  }),

  http.post<never, { name: string }, TagResponse>(
    "/api/finance/tags",
    async ({ request }) => {
      await simulateLatency();
      const body = await request.json();
      const tag: Tag = {
        id: crypto.randomUUID(),
        name: body.name,
        isDefault: false,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      return HttpResponse.json({ tag }, { status: 201 });
    },
  ),

  http.put<{ id: string }, { name: string }, TagResponse>(
    "/api/finance/tags/:id",
    async ({ request, params }) => {
      await simulateLatency();
      const body = await request.json();
      const existing = mockTags.find((tag) => tag.id === params.id);
      const base: Tag = existing ?? {
        id: params.id,
        name: "",
        isDefault: false,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      const tag: Tag = { ...base, name: body.name, updatedAt: new Date().toISOString() };
      return HttpResponse.json({ tag });
    },
  ),

  http.delete<{ id: string }, never, ApiError>(
    "/api/finance/tags/:id",
    async ({ params }) => {
      await simulateLatency();
      const tag = mockTags.find((t) => t.id === params.id);
      if (tag?.isDefault) {
        return HttpResponse.json(
          { code: "DEFAULT_TAG", message: "Default tags cannot be deleted" },
          { status: 400 },
        );
      }
      return new HttpResponse(null, { status: 204 });
    },
  ),
];
