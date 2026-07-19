import { describe, it, expect, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { usePrefersReducedMotion } from "../hooks/usePrefersReducedMotion";

interface FakeMediaQuery {
  matches: boolean;
  listeners: ((event: MediaQueryListEvent) => void)[];
}

/** Install a matchMedia stub whose change listener can be fired from the test. */
function stubMatchMedia(initialMatches: boolean): FakeMediaQuery {
  const state: FakeMediaQuery = { matches: initialMatches, listeners: [] };
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: state.matches,
    media: query,
    onchange: null,
    addEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) =>
      state.listeners.push(listener),
    removeEventListener: (
      _: string,
      listener: (event: MediaQueryListEvent) => void,
    ) => {
      state.listeners = state.listeners.filter((l) => l !== listener);
    },
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
  return state;
}

afterEach(() => {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
});

describe("usePrefersReducedMotion", () => {
  it("reads the preference on mount", () => {
    stubMatchMedia(true);

    const { result } = renderHook(() => usePrefersReducedMotion());

    expect(result.current).toBe(true);
  });

  it("defaults to false when the user has no preference", () => {
    stubMatchMedia(false);

    const { result } = renderHook(() => usePrefersReducedMotion());

    expect(result.current).toBe(false);
  });

  it("updates when the preference changes", () => {
    const media = stubMatchMedia(false);

    const { result } = renderHook(() => usePrefersReducedMotion());
    expect(result.current).toBe(false);

    act(() => {
      for (const listener of media.listeners) {
        listener({ matches: true } as MediaQueryListEvent);
      }
    });

    expect(result.current).toBe(true);
  });
});
