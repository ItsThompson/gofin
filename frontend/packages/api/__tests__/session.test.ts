import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { consumeReturnToPath, handleSessionExpiry } from "../src/session";

describe("consumeReturnToPath", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("returns null when no return-to path is stored", () => {
    expect(consumeReturnToPath()).toBeNull();
  });

  it("returns stored path and clears it from sessionStorage", () => {
    sessionStorage.setItem("gofin_return_to", "/dashboard");

    const path = consumeReturnToPath();

    expect(path).toBe("/dashboard");
    expect(sessionStorage.getItem("gofin_return_to")).toBeNull();
  });

  it("only clears the return-to key, not other storage", () => {
    sessionStorage.setItem("gofin_return_to", "/settings");
    sessionStorage.setItem("other_key", "other_value");

    consumeReturnToPath();

    expect(sessionStorage.getItem("other_key")).toBe("other_value");
  });
});

describe("handleSessionExpiry", () => {
  const originalLocation = window.location;

  beforeEach(() => {
    sessionStorage.clear();
    Object.defineProperty(window, "location", {
      writable: true,
      value: { pathname: "/budget/2024/01", href: "" },
    });
  });

  afterEach(() => {
    Object.defineProperty(window, "location", {
      writable: true,
      value: originalLocation,
    });
  });

  it("saves current path to sessionStorage", () => {
    handleSessionExpiry();

    expect(sessionStorage.getItem("gofin_return_to")).toBe("/budget/2024/01");
  });

  it("redirects to login with expired=true", () => {
    handleSessionExpiry();

    expect(window.location.href).toBe("/login?expired=true");
  });
});
