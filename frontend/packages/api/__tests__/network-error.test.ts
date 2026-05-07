import { describe, it, expect } from "vitest";
import { isNetworkError, NETWORK_ERROR_MESSAGE } from "../src/hooks/useApiToast";

describe("isNetworkError", () => {
  it("returns true for TypeError with 'Failed to fetch' message", () => {
    const error = new TypeError("Failed to fetch");
    expect(isNetworkError(error)).toBe(true);
  });

  it("returns true for TypeError with 'NetworkError' message", () => {
    const error = new TypeError("NetworkError when attempting to fetch resource");
    expect(isNetworkError(error)).toBe(true);
  });

  it("returns true for TypeError with 'Load failed' message (Safari)", () => {
    const error = new TypeError("Load failed");
    expect(isNetworkError(error)).toBe(true);
  });

  it("returns true for TypeError containing 'fetch' (case insensitive)", () => {
    const error = new TypeError("The fetch operation was aborted");
    expect(isNetworkError(error)).toBe(true);
  });

  it("returns false for TypeError with unrelated message", () => {
    const error = new TypeError("Cannot read properties of undefined");
    expect(isNetworkError(error)).toBe(false);
  });

  it("returns false for non-TypeError errors", () => {
    const error = new Error("Failed to fetch");
    expect(isNetworkError(error)).toBe(false);
  });

  it("returns false for RangeError even with network-like message", () => {
    const error = new RangeError("network issue");
    expect(isNetworkError(error)).toBe(false);
  });

  it("returns false for null", () => {
    expect(isNetworkError(null)).toBe(false);
  });

  it("returns false for undefined", () => {
    expect(isNetworkError(undefined)).toBe(false);
  });

  it("returns false for string errors", () => {
    expect(isNetworkError("Failed to fetch")).toBe(false);
  });

  it("returns false for plain objects", () => {
    expect(isNetworkError({ message: "Failed to fetch" })).toBe(false);
  });
});

describe("NETWORK_ERROR_MESSAGE", () => {
  it("contains user-friendly connection lost text", () => {
    expect(NETWORK_ERROR_MESSAGE).toBe(
      "Connection lost. Check your internet and try again.",
    );
  });
});
