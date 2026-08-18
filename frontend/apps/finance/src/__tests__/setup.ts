import "@testing-library/jest-dom/vitest";
import { beforeAll } from "vitest";
import { loadSupportedCurrencies } from "@gofin/core";
import { mockCurrencyCatalog } from "@gofin/test-utils";

// The currency catalog is populated by the app shell at runtime; seed it here
// so tests exercise formatting and selection against the real catalog data.
beforeAll(async () => {
  await loadSupportedCurrencies(async () => mockCurrencyCatalog, []);
});

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
