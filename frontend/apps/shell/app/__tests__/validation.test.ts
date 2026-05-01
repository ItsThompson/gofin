import { describe, it, expect } from "vitest";
import {
  validatePassword,
  validateEmail,
  validateUsername,
} from "@/lib/validation";

describe("validatePassword", () => {
  it("returns null for valid password", () => {
    expect(validatePassword("Password1")).toBeNull();
    expect(validatePassword("Abcdefg1")).toBeNull();
    expect(validatePassword("MyP4ssword!")).toBeNull();
  });

  it("rejects passwords shorter than 8 characters", () => {
    expect(validatePassword("Abc1")).not.toBeNull();
    expect(validatePassword("Pass1")).not.toBeNull();
    expect(validatePassword("Ab1")).not.toBeNull();
  });

  it("rejects passwords without uppercase letter", () => {
    expect(validatePassword("password1")).not.toBeNull();
    expect(validatePassword("abcdefgh1")).not.toBeNull();
  });

  it("rejects passwords without lowercase letter", () => {
    expect(validatePassword("PASSWORD1")).not.toBeNull();
    expect(validatePassword("ABCDEFGH1")).not.toBeNull();
  });

  it("rejects passwords without digit", () => {
    expect(validatePassword("Passwordd")).not.toBeNull();
    expect(validatePassword("ABCDEfgh")).not.toBeNull();
  });

  it("returns consistent error message matching server", () => {
    const expected =
      "Password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit";
    expect(validatePassword("short")).toBe(expected);
    expect(validatePassword("nouppercase1")).toBe(expected);
    expect(validatePassword("NOLOWERCASE1")).toBe(expected);
    expect(validatePassword("NoDigitHere")).toBe(expected);
  });
});

describe("validateEmail", () => {
  it("returns null for valid emails", () => {
    expect(validateEmail("test@example.com")).toBeNull();
    expect(validateEmail("user@domain.co")).toBeNull();
  });

  it("rejects empty email", () => {
    expect(validateEmail("")).not.toBeNull();
    expect(validateEmail("   ")).not.toBeNull();
  });

  it("rejects invalid email format", () => {
    expect(validateEmail("notanemail")).not.toBeNull();
    expect(validateEmail("missing@tld")).not.toBeNull();
    expect(validateEmail("@nouser.com")).not.toBeNull();
  });
});

describe("validateUsername", () => {
  it("returns null for valid usernames", () => {
    expect(validateUsername("testuser")).toBeNull();
    expect(validateUsername("ab")).toBeNull();
  });

  it("rejects empty username", () => {
    expect(validateUsername("")).not.toBeNull();
    expect(validateUsername("   ")).not.toBeNull();
  });

  it("rejects username shorter than 2 characters", () => {
    expect(validateUsername("a")).not.toBeNull();
  });
});
