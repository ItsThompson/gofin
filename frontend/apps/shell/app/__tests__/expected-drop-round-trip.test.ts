import { describe, it, expect, afterAll } from "vitest";
import * as Sentry from "@sentry/react-router";
import { ApiRequestError, isNetworkError, reportError } from "@gofin/api";
import { clientOptions } from "../../sentry.options.mjs";

/**
 * The only assertion that spans both halves of the `expected` contract: the real
 * `reportError` emits the tag and the real `clientOptions` chain compares it.
 * Both sides' own tests build a fixture on one side of that seam, and the
 * frontend-unit layer mocks the SDK so `beforeSend` never runs inside a
 * `reportError` test.
 *
 * A stub transport is the seam instead, so the real SDK applies the scope, runs
 * `beforeSend`, and the assertion is on what would have gone over the wire.
 */

interface SentEvent {
  tags?: Record<string, string>;
  level?: string;
  exception?: { values?: { value?: string }[] };
}

type EnvelopeItem = [{ type: string }, SentEvent];
type Envelope = [Record<string, unknown>, EnvelopeItem[]];

const envelopes: Envelope[] = [];

Sentry.init({
  ...clientOptions({
    dsn: "https://publickey@o1.ingest.us.sentry.io/2",
    release: "gofin-web@0123456789abcdef",
    isNetworkError,
  }),
  transport: () => ({
    send: (envelope: unknown) => {
      envelopes.push(envelope as Envelope);
      return Promise.resolve({});
    },
    flush: () => Promise.resolve(true),
  }),
});

/** Error events that survived beforeSend, with session envelopes filtered out. */
function sentErrorEvents(): SentEvent[] {
  return envelopes.flatMap(([, items]) =>
    items
      .filter(([header]) => header.type === "event")
      .map(([, event]) => event),
  );
}

afterAll(async () => {
  await Sentry.close(2000);
});

describe("the expected drop, from reportError to beforeSend", () => {
  it("drops a 422 and sends the 500 beside it", async () => {
    reportError(
      new ApiRequestError(422, {
        code: "VALIDATION_ERROR",
        message: "Amount must be positive",
      }),
      {
        kind: "validation",
        level: "warning",
        expected: true,
        op: "expense.create",
        domain: "expenses",
      },
    );
    reportError(
      new ApiRequestError(500, {
        code: "INTERNAL_ERROR",
        message: "Server error",
      }),
      { kind: "upstream", op: "expense.create", domain: "expenses" },
    );

    await Sentry.flush(2000);

    const events = sentErrorEvents();
    expect(events).toHaveLength(1);
    expect(events[0].exception?.values?.[0]?.value).toBe("Server error");
    expect(events[0].tags).toMatchObject({
      app: "gofin-web",
      service: "web",
      error_kind: "upstream",
      operation: "expense.create",
      domain: "expenses",
    });
    expect(events[0].tags).not.toHaveProperty("expected");
  });
});
