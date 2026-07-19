import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

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
