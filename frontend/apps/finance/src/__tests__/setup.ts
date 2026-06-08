import "@testing-library/jest-dom/vitest";

// Radix UI Select uses pointer capture and scroll APIs not available in JSDOM.
// Provide stubs to prevent runtime errors during tests.
if (typeof Element.prototype.hasPointerCapture === "undefined") {
  Element.prototype.hasPointerCapture = () => false;
  Element.prototype.setPointerCapture = () => {};
  Element.prototype.releasePointerCapture = () => {};
}
if (typeof Element.prototype.scrollIntoView === "undefined") {
  Element.prototype.scrollIntoView = () => {};
}
