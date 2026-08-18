import "@testing-library/jest-dom/vitest";
import { beforeAll, vi } from "vitest";
import { loadSupportedCurrencies } from "@gofin/core";
import { mockCurrencyCatalog } from "@gofin/test-utils";

// The currency catalog is populated by the app shell at runtime; seed it here
// so onboarding and finance tests render options against the real catalog data.
beforeAll(async () => {
  await loadSupportedCurrencies(async () => mockCurrencyCatalog, []);
});

/**
 * jsdom does not implement several DOM APIs that framer-motion and radix-ui
 * read at runtime. Stub them once here so the reduced-motion path and the
 * radix DropdownMenu (portal + popper) work under the test runner.
 */

// framer-motion's useReducedMotion reads matchMedia. Default to "no preference"
// (matches=false); a test can reassign window.matchMedia to force reduced
// motion on. addEventListener/removeEventListener are no-ops.
if (!window.matchMedia) {
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
}

// radix popper measures with ResizeObserver.
if (!window.ResizeObserver) {
  window.ResizeObserver = class {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  };
}

// radix menu focuses items via scrollIntoView and reads pointer-capture state.
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = vi.fn();
}
if (!Element.prototype.hasPointerCapture) {
  Element.prototype.hasPointerCapture = vi.fn(() => false);
}
if (!Element.prototype.setPointerCapture) {
  Element.prototype.setPointerCapture = vi.fn();
}
if (!Element.prototype.releasePointerCapture) {
  Element.prototype.releasePointerCapture = vi.fn();
}
