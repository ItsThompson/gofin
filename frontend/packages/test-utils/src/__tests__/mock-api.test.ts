import { describe, it, expect } from "vitest";
import { createMockApi, mockSequence, expectCalled } from "../mock-api";

describe("createMockApi", () => {
  describe("URL matching", () => {
    it("matches URLs by substring", async () => {
      const mockFetch = createMockApi({
        "/api/finance/periods": { period: { id: "p1" } },
      });

      const response = await mockFetch("http://localhost:3000/api/finance/periods?year=2026");
      const data = await response.json();

      expect(response.status).toBe(200);
      expect(data).toEqual({ period: { id: "p1" } });
    });

    it("first matching route wins when multiple patterns match", async () => {
      const mockFetch = createMockApi({
        "/api/finance/periods/current": { period: { id: "current" } },
        "/api/finance/periods": { periods: [] },
      });

      const response = await mockFetch("http://localhost:3000/api/finance/periods/current");
      const data = await response.json();

      expect(data).toEqual({ period: { id: "current" } });
    });

    it("matches second route when first does not match", async () => {
      const mockFetch = createMockApi({
        "/api/finance/periods/current": { period: { id: "current" } },
        "/api/finance/summary": { summary: { total: 100 } },
      });

      const response = await mockFetch("http://localhost:3000/api/finance/summary");
      const data = await response.json();

      expect(data).toEqual({ summary: { total: 100 } });
    });
  });

  describe("rejection for unmatched URLs", () => {
    it("rejects with an error naming the full URL and method", async () => {
      const mockFetch = createMockApi({
        "/api/known": { ok: true },
      });

      await expect(
        mockFetch("http://localhost:3000/api/unknown/path", { method: "POST" }),
      ).rejects.toThrow("No mock route for: POST http://localhost:3000/api/unknown/path");
    });

    it("defaults to GET method in rejection message", async () => {
      const mockFetch = createMockApi({});

      await expect(
        mockFetch("http://localhost:3000/api/test"),
      ).rejects.toThrow("No mock route for: GET http://localhost:3000/api/test");
    });
  });

  describe("stable responses", () => {
    it("returns the same response on every call to the same URL", async () => {
      const mockFetch = createMockApi({
        "/api/data": { value: 42 },
      });

      const response1 = await mockFetch("http://localhost:3000/api/data");
      const response2 = await mockFetch("http://localhost:3000/api/data");

      const data1 = await response1.json();
      const data2 = await response2.json();

      expect(data1).toEqual({ value: 42 });
      expect(data2).toEqual({ value: 42 });
    });
  });

  describe("MockResponse with status and headers", () => {
    it("supports full MockResponse with custom status", async () => {
      const mockFetch = createMockApi({
        "/api/auth/me": { status: 401, body: { code: "UNAUTHORIZED", message: "Not authenticated" } },
      });

      const response = await mockFetch("http://localhost:3000/api/auth/me");

      expect(response.status).toBe(401);
      const data = await response.json();
      expect(data).toEqual({ code: "UNAUTHORIZED", message: "Not authenticated" });
    });

    it("supports custom headers", async () => {
      const mockFetch = createMockApi({
        "/api/data": { status: 200, body: {}, headers: { "x-custom": "value" } },
      });

      const response = await mockFetch("http://localhost:3000/api/data");

      expect(response.headers.get("x-custom")).toBe("value");
      expect(response.headers.get("content-type")).toBe("application/json");
    });
  });

  describe("no global state between instances", () => {
    it("each createMockApi call produces an independent mock", async () => {
      const mock1 = createMockApi({ "/api/a": { from: "mock1" } });
      const mock2 = createMockApi({ "/api/b": { from: "mock2" } });

      await expect(mock1("http://localhost:3000/api/b")).rejects.toThrow();
      await expect(mock2("http://localhost:3000/api/a")).rejects.toThrow();

      const resp1 = await mock1("http://localhost:3000/api/a");
      const resp2 = await mock2("http://localhost:3000/api/b");

      expect(await resp1.json()).toEqual({ from: "mock1" });
      expect(await resp2.json()).toEqual({ from: "mock2" });
    });
  });
});

describe("mockSequence", () => {
  it("returns different responses on sequential calls", async () => {
    const mockFetch = createMockApi({
      "/api/auth/refresh": mockSequence([
        { body: { token: "new-token" } },
        { status: 401, body: { code: "EXPIRED", message: "Token expired" } },
      ]),
    });

    const response1 = await mockFetch("http://localhost:3000/api/auth/refresh");
    expect(response1.status).toBe(200);
    expect(await response1.json()).toEqual({ token: "new-token" });

    const response2 = await mockFetch("http://localhost:3000/api/auth/refresh");
    expect(response2.status).toBe(401);
    expect(await response2.json()).toEqual({ code: "EXPIRED", message: "Token expired" });
  });

  it("rejects when sequence is exhausted", async () => {
    const mockFetch = createMockApi({
      "/api/poll": mockSequence([{ body: { status: "pending" } }]),
    });

    await mockFetch("http://localhost:3000/api/poll");

    await expect(
      mockFetch("http://localhost:3000/api/poll"),
    ).rejects.toThrow('Mock sequence exhausted for pattern "/api/poll" after 1 calls');
  });
});

describe("expectCalled", () => {
  it("passes when the URL was called", async () => {
    const mockFetch = createMockApi({
      "/api/finance/periods": { period: {} },
    });

    await mockFetch("http://localhost:3000/api/finance/periods");

    expect(() => expectCalled(mockFetch, "/api/finance/periods")).not.toThrow();
  });

  it("fails when the URL was not called", async () => {
    const mockFetch = createMockApi({
      "/api/finance/periods": { period: {} },
    });

    expect(() => expectCalled(mockFetch, "/api/finance/periods")).toThrow(
      /Expected at least one call matching/,
    );
  });

  it("asserts method matches", async () => {
    const mockFetch = createMockApi({
      "/api/finance/periods": { period: {} },
    });

    await mockFetch("http://localhost:3000/api/finance/periods", { method: "POST", body: JSON.stringify({}) });

    expect(() =>
      expectCalled(mockFetch, "/api/finance/periods", { method: "POST" }),
    ).not.toThrow();

    expect(() =>
      expectCalled(mockFetch, "/api/finance/periods", { method: "DELETE" }),
    ).toThrow();
  });

  it("asserts body matches", async () => {
    const mockFetch = createMockApi({
      "/api/finance/periods": { period: {} },
    });

    await mockFetch("http://localhost:3000/api/finance/periods", {
      method: "POST",
      body: JSON.stringify({ year: 2026, month: 5 }),
    });

    expect(() =>
      expectCalled(mockFetch, "/api/finance/periods", {
        method: "POST",
        body: { year: 2026, month: 5 },
      }),
    ).not.toThrow();

    expect(() =>
      expectCalled(mockFetch, "/api/finance/periods", {
        method: "POST",
        body: { year: 2025, month: 1 },
      }),
    ).toThrow();
  });

  it("matches by substring in URL pattern", async () => {
    const mockFetch = createMockApi({
      "/api/finance/periods": { period: {} },
    });

    await mockFetch("http://localhost:3000/api/finance/periods?year=2026&month=5");

    expect(() => expectCalled(mockFetch, "/api/finance/periods")).not.toThrow();
  });
});
