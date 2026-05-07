import { expect } from "vitest";

/** Response definition for a mocked endpoint. */
export interface MockResponse {
  /** HTTP status code. Defaults to 200. */
  status?: number;
  /** Response body. Serialized to JSON automatically. */
  body: unknown;
  /** Optional headers. */
  headers?: Record<string, string>;
}

/** A function matching fetch's signature, for use as global.fetch replacement. */
export type MockFetch = ((
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>) & {
  /** Internal call log for assertion purposes. */
  _calls: Array<{ url: string; method: string; body: unknown }>;
};

/** Marker for sequence responses. */
const SEQUENCE_MARKER = Symbol("mockSequence");

interface SequenceResponse {
  [SEQUENCE_MARKER]: true;
  responses: MockResponse[];
}

/**
 * Configuration for the mock router. Keys are URL patterns (substring match).
 * Values are either a response body (assumes 200) or a full MockResponse.
 */
export type MockRoutes = Record<string, unknown | MockResponse>;

function isFullMockResponse(value: unknown): value is MockResponse {
  return (
    typeof value === "object" &&
    value !== null &&
    "body" in value
  );
}

function isSequenceResponse(value: unknown): value is SequenceResponse {
  return (
    typeof value === "object" &&
    value !== null &&
    SEQUENCE_MARKER in value
  );
}

function buildResponse(definition: unknown): Response {
  let status: number;
  let body: unknown;
  let headers: Record<string, string> = {};

  if (isFullMockResponse(definition)) {
    status = definition.status ?? 200;
    body = definition.body;
    headers = definition.headers ?? {};
  } else {
    status = 200;
    body = definition;
  }

  const responseHeaders = new Headers({ "content-type": "application/json", ...headers });

  return new Response(JSON.stringify(body), {
    status,
    headers: responseHeaders,
  });
}

/**
 * Create a mock fetch function that routes by URL pattern.
 *
 * Behavior:
 * - Matches URLs by substring: '/api/finance/periods/current' matches
 *   'http://localhost:3000/api/finance/periods/current?year=2026&month=5'
 * - First matching route wins (routes are checked in insertion order)
 * - Unmatched URLs reject with: "No mock route for: GET /api/unknown/path"
 * - Each route returns the same response on every call (stable by default)
 */
export function createMockApi(routes: MockRoutes): MockFetch {
  const sequenceCounters: Record<string, number> = {};
  const calls: Array<{ url: string; method: string; body: unknown }> = [];

  const mockFetch: MockFetch = async (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> => {
    const url = typeof input === "string" ? input : input.toString();
    const method = (init?.method ?? "GET").toUpperCase();

    let parsedBody: unknown = undefined;
    if (init?.body) {
      try {
        parsedBody = JSON.parse(init.body as string);
      } catch {
        parsedBody = init.body;
      }
    }

    calls.push({ url, method, body: parsedBody });

    const patterns = Object.keys(routes);
    for (const pattern of patterns) {
      if (url.includes(pattern)) {
        const definition = routes[pattern];

        if (isSequenceResponse(definition)) {
          const index = sequenceCounters[pattern] ?? 0;
          sequenceCounters[pattern] = index + 1;

          const response = definition.responses[index];
          if (!response) {
            return Promise.reject(
              new Error(
                `Mock sequence exhausted for pattern "${pattern}" after ${index} calls`,
              ),
            );
          }
          return buildResponse(response);
        }

        return buildResponse(definition);
      }
    }

    return Promise.reject(
      new Error(`No mock route for: ${method} ${url}`),
    );
  };

  mockFetch._calls = calls;
  return mockFetch;
}

/**
 * Create a mock that returns different responses on sequential calls.
 * Use sparingly: only when testing retry behavior or polling.
 */
export function mockSequence(responses: MockResponse[]): SequenceResponse {
  return {
    [SEQUENCE_MARKER]: true,
    responses,
  };
}

/**
 * Assert that a specific URL was called with expected options.
 * Used after component interactions to verify correct API calls.
 */
export function expectCalled(
  mockFetch: MockFetch,
  urlPattern: string,
  expectedOptions?: { method?: string; body?: unknown },
): void {
  const matchingCalls = mockFetch._calls.filter((call) =>
    call.url.includes(urlPattern),
  );

  expect(
    matchingCalls.length,
    `Expected at least one call matching "${urlPattern}", but found none. Calls made: ${JSON.stringify(mockFetch._calls.map((call) => `${call.method} ${call.url}`))}`,
  ).toBeGreaterThan(0);

  if (expectedOptions) {
    const hasMatch = matchingCalls.some((call) => {
      if (expectedOptions.method && call.method !== expectedOptions.method.toUpperCase()) {
        return false;
      }
      if (expectedOptions.body !== undefined) {
        try {
          expect(call.body).toEqual(expectedOptions.body);
          return true;
        } catch {
          return false;
        }
      }
      return true;
    });

    expect(
      hasMatch,
      `Expected a call to "${urlPattern}" with ${JSON.stringify(expectedOptions)}, but no matching call found. Matching calls: ${JSON.stringify(matchingCalls)}`,
    ).toBe(true);
  }
}
