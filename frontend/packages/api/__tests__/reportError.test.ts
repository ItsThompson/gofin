import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

/** The subset of Sentry's CaptureContext that reportError is allowed to send. */
interface CapturedContext {
  level?: string;
  tags?: Record<string, string>;
  fingerprint?: string[];
  contexts?: Record<string, Record<string, unknown>>;
}

const { captureException, EVENT_ID } = vi.hoisted(() => {
  const EVENT_ID = "1f0c9a3e5b7d4f2ab6c8d0e2f4a6b8c0";
  return {
    EVENT_ID,
    captureException: vi.fn<
      (error: unknown, context?: CapturedContext) => string
    >(() => EVENT_ID),
  };
});

vi.mock("@sentry/react-router", () => ({ captureException }));

import { reportError } from "../src/errors/report";

/** The single recorded call, so every assertion also proves exactly one capture. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

describe("reportError", () => {
  beforeEach(() => {
    captureException.mockClear();
  });

  describe("never re-wraps its input", () => {
    it("passes a string through instead of converting it to an Error", () => {
      reportError("something went wrong");

      const { error } = onlyCapture();
      expect(error).toBe("something went wrong");
      expect(error).not.toBeInstanceOf(Error);
    });

    it("leaves a thrown Error's stack byte-identical", () => {
      let thrown: unknown;
      try {
        throw new Error("kaboom");
      } catch (err) {
        thrown = err;
      }
      const stackBefore = (thrown as Error).stack;
      expect(stackBefore).toMatch(/^Error: kaboom\n/);

      reportError(thrown);

      const { error } = onlyCapture();
      expect(error).toBe(thrown);
      expect((thrown as Error).stack).toBe(stackBefore);
    });

    it("passes a plain object through untouched", () => {
      const raw = { code: "E_WEIRD" };

      reportError(raw);

      const { error } = onlyCapture();
      expect(error).toBe(raw);
    });

    it("passes null through rather than silently dropping it", () => {
      reportError(null);

      expect(onlyCapture().error).toBeNull();
    });

    it("passes undefined through rather than silently dropping it", () => {
      reportError(undefined);

      expect(onlyCapture().error).toBeUndefined();
    });
  });

  describe("default resolution", () => {
    it("sends level error, kind internal, and no operation or domain tag", () => {
      const err = new Error("boom");

      reportError(err);

      expect(captureException).toHaveBeenCalledWith(err, {
        level: "error",
        tags: { error_kind: "internal" },
        fingerprint: ["{{ default }}", "internal"],
        contexts: undefined,
      });
    });

    it("returns the SDK event id", () => {
      expect(reportError(new Error("boom"))).toBe(EVENT_ID);
    });

    it("honours an explicit level and kind", () => {
      reportError(new Error("boom"), { level: "warning", kind: "validation" });

      const { context } = onlyCapture();
      expect(context.level).toBe("warning");
      expect(context.tags).toEqual({ error_kind: "validation" });
    });
  });

  describe("tags", () => {
    it("emits operation and domain when both are given", () => {
      reportError(new Error("boom"), {
        kind: "upstream",
        op: "expense.create",
        domain: "expenses",
      });

      expect(onlyCapture().context.tags).toEqual({
        error_kind: "upstream",
        operation: "expense.create",
        domain: "expenses",
      });
    });

    it("emits expected as the string \"true\", not a boolean", () => {
      reportError(new Error("boom"), { expected: true });

      const expected = onlyCapture().context.tags?.expected;
      expect(expected).toBe("true");
      expect(typeof expected).toBe("string");
    });

    it("omits expected when it is false", () => {
      reportError(new Error("boom"), { expected: false });

      expect(onlyCapture().context.tags).toEqual({ error_kind: "internal" });
    });

    it("stringifies numeric and boolean tag values", () => {
      reportError(new Error("boom"), {
        tags: { http_status: 503, retried: false },
      });

      expect(onlyCapture().context.tags).toEqual({
        error_kind: "internal",
        http_status: "503",
        retried: "false",
      });
    });

    it("replaces newlines in a tag value with spaces", () => {
      reportError(new Error("boom"), { tags: { target: "line1\nline2\n" } });

      expect(onlyCapture().context.tags?.target).toBe("line1 line2 ");
    });

    it("truncates a tag value at 200 characters", () => {
      reportError(new Error("boom"), { tags: { target: "x".repeat(250) } });

      expect(onlyCapture().context.tags?.target).toBe("x".repeat(200));
    });

    it("does not let a caller tag override the taxonomy", () => {
      reportError(new Error("boom"), {
        kind: "network",
        op: "chunk.load",
        domain: "platform",
        expected: true,
        tags: {
          error_kind: "validation",
          operation: "nope",
          domain: "nope",
          expected: false,
        },
      });

      expect(onlyCapture().context.tags).toEqual({
        error_kind: "network",
        operation: "chunk.load",
        domain: "platform",
        expected: "true",
      });
    });
  });

  describe("fingerprint", () => {
    it("derives op/kind by default", () => {
      reportError(new Error("boom"), {
        kind: "upstream",
        op: "expense.create",
      });

      expect(onlyCapture().context.fingerprint).toEqual([
        "{{ default }}",
        "expense.create/upstream",
      ]);
    });

    it("falls back to the kind alone when no op is given", () => {
      reportError(new Error("boom"), { kind: "timeout" });

      expect(onlyCapture().context.fingerprint).toEqual([
        "{{ default }}",
        "timeout",
      ]);
    });

    it("uses an explicit groupKey over the derived one", () => {
      reportError(new Error("boom"), {
        kind: "upstream",
        op: "expense.create",
        groupKey: "expense_write_failed",
      });

      expect(onlyCapture().context.fingerprint).toEqual([
        "{{ default }}",
        "expense_write_failed",
      ]);
    });

    it("collapses to the key alone for groupExact with a key", () => {
      reportError(new Error("boom"), {
        kind: "network",
        op: "chunk.load",
        groupKey: "chunk_load_failed",
        groupExact: true,
      });

      expect(onlyCapture().context.fingerprint).toEqual(["chunk_load_failed"]);
    });

    it("ignores groupExact without a key rather than emitting an empty fingerprint", () => {
      reportError(new Error("boom"), {
        kind: "network",
        op: "chunk.load",
        groupExact: true,
      });

      expect(onlyCapture().context.fingerprint).toEqual([
        "{{ default }}",
        "chunk.load/network",
      ]);
    });

    it("ignores groupExact with an empty key rather than emitting an empty element", () => {
      reportError(new Error("boom"), {
        kind: "network",
        groupKey: "",
        groupExact: true,
      });

      expect(onlyCapture().context.fingerprint).toEqual([
        "{{ default }}",
        "network",
      ]);
    });
  });

  describe("data", () => {
    it("lands under contexts.gofin and never under extra", () => {
      reportError(new Error("boom"), {
        data: { expenseId: "exp-1", fields: ["amount"] },
      });

      const { context } = onlyCapture();
      expect(context.contexts).toEqual({
        gofin: { expenseId: "exp-1", fields: ["amount"] },
      });
      expect(context).not.toHaveProperty("extra");
    });

    it("sends no gofin context block when no data is given", () => {
      reportError(new Error("boom"));

      expect(onlyCapture().context.contexts).toBeUndefined();
    });
  });

  describe("uninitialized SDK", () => {
    const consoleMethods = ["error", "warn", "log", "info", "debug"] as const;
    const spies = new Map<string, ReturnType<typeof vi.spyOn>>();

    beforeEach(() => {
      for (const method of consoleMethods) {
        spies.set(method, vi.spyOn(console, method).mockImplementation(() => {}));
      }
    });

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it("neither throws nor logs", () => {
      expect(() => reportError("no dsn configured")).not.toThrow();

      for (const method of consoleMethods) {
        expect(spies.get(method)).not.toHaveBeenCalled();
      }
    });
  });
});
