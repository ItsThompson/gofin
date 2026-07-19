import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

/**
 * jsdom does not implement the DOM APIs radix-ui popper/menu components read
 * at runtime. Stub them so portal + popper components (DropdownMenu, Select)
 * open and focus under the test runner.
 */
if (!window.ResizeObserver) {
  window.ResizeObserver = class {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  };
}
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
