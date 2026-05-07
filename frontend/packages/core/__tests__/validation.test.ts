import { describe, it, expect } from "vitest";
import {
  validateEDSSplit,
  validatePassword,
  validateEmail,
  validateUsername,
} from "../src/validation";

describe("validateEDSSplit", () => {
  describe("non-negative validation", () => {
    it("returns error when essentials is negative", () => {
      expect(validateEDSSplit(-10, 60, 50)).toBe(
        "Percentages must be non-negative",
      );
    });

    it("returns error when desires is negative", () => {
      expect(validateEDSSplit(60, -10, 50)).toBe(
        "Percentages must be non-negative",
      );
    });

    it("returns error when savings is negative", () => {
      expect(validateEDSSplit(60, 50, -10)).toBe(
        "Percentages must be non-negative",
      );
    });

    it("returns non-negative error before sum error when both fail", () => {
      // -10 + 60 + 60 = 110 (fails both checks), non-negative takes priority
      expect(validateEDSSplit(-10, 60, 60)).toBe(
        "Percentages must be non-negative",
      );
    });
  });

  describe("zero values (valid: zero is non-negative)", () => {
    it("returns null for 0/0/100 split", () => {
      expect(validateEDSSplit(0, 0, 100)).toBeNull();
    });

    it("returns null for 100/0/0 split", () => {
      expect(validateEDSSplit(100, 0, 0)).toBeNull();
    });

    it("returns null for 0/100/0 split", () => {
      expect(validateEDSSplit(0, 100, 0)).toBeNull();
    });
  });

  describe("sum validation", () => {
    it("returns null for valid 50/30/20 split", () => {
      expect(validateEDSSplit(50, 30, 20)).toBeNull();
    });

    it("returns null for valid 33/34/33 split", () => {
      expect(validateEDSSplit(33, 34, 33)).toBeNull();
    });

    it("returns error when sum is less than 100", () => {
      expect(validateEDSSplit(50, 30, 19)).toBe(
        "Percentages must sum to 100% (currently 99%)",
      );
    });

    it("returns error when sum exceeds 100", () => {
      expect(validateEDSSplit(50, 30, 21)).toBe(
        "Percentages must sum to 100% (currently 101%)",
      );
    });

    it("returns error for all zeros", () => {
      expect(validateEDSSplit(0, 0, 0)).toBe(
        "Percentages must sum to 100% (currently 0%)",
      );
    });
  });
});

describe("validatePassword", () => {
  describe("valid passwords", () => {
    it("returns null for password meeting all criteria", () => {
      expect(validatePassword("Password1")).toBeNull();
    });

    it("returns null for longer valid password", () => {
      expect(validatePassword("MySecureP4ss!")).toBeNull();
    });

    it("returns null for exactly 8 characters meeting all rules", () => {
      expect(validatePassword("Abcdefg1")).toBeNull();
    });
  });

  describe("length requirement", () => {
    it("rejects passwords shorter than 8 characters", () => {
      expect(validatePassword("Abc1")).toBe(
        "Password must be at least 8 characters",
      );
    });

    it("rejects 7-character password", () => {
      expect(validatePassword("Abcdef1")).toBe(
        "Password must be at least 8 characters",
      );
    });

    it("rejects empty password", () => {
      expect(validatePassword("")).toBe(
        "Password must be at least 8 characters",
      );
    });
  });

  describe("uppercase requirement", () => {
    it("rejects password without uppercase letter", () => {
      expect(validatePassword("password1")).toBe(
        "Password must contain at least one uppercase letter",
      );
    });
  });

  describe("lowercase requirement", () => {
    it("rejects password without lowercase letter", () => {
      expect(validatePassword("PASSWORD1")).toBe(
        "Password must contain at least one lowercase letter",
      );
    });
  });

  describe("digit requirement", () => {
    it("rejects password without digit", () => {
      expect(validatePassword("Passwordd")).toBe(
        "Password must contain at least one digit",
      );
    });
  });

  describe("rule priority", () => {
    it("reports length error first even when other rules also fail", () => {
      expect(validatePassword("abc")).toBe(
        "Password must be at least 8 characters",
      );
    });

    it("reports uppercase error before lowercase and digit errors", () => {
      expect(validatePassword("alllowercase1")).toBe(
        "Password must contain at least one uppercase letter",
      );
    });
  });
});

describe("validateEmail", () => {
  describe("valid emails", () => {
    it("returns null for standard email format", () => {
      expect(validateEmail("test@example.com")).toBeNull();
    });

    it("returns null for email with subdomain", () => {
      expect(validateEmail("user@mail.domain.co")).toBeNull();
    });

    it("returns null for email with plus addressing", () => {
      expect(validateEmail("user+tag@example.com")).toBeNull();
    });
  });

  describe("empty input", () => {
    it("rejects empty string", () => {
      expect(validateEmail("")).toBe("Email is required");
    });

    it("rejects whitespace-only input", () => {
      expect(validateEmail("   ")).toBe("Email is required");
    });
  });

  describe("invalid format", () => {
    it("rejects string without @", () => {
      expect(validateEmail("notanemail")).toBe(
        "Please enter a valid email address",
      );
    });

    it("rejects email without TLD", () => {
      expect(validateEmail("missing@tld")).toBe(
        "Please enter a valid email address",
      );
    });

    it("rejects email without local part", () => {
      expect(validateEmail("@nouser.com")).toBe(
        "Please enter a valid email address",
      );
    });
  });
});

describe("validateUsername", () => {
  describe("valid usernames", () => {
    it("returns null for valid username", () => {
      expect(validateUsername("testuser")).toBeNull();
    });

    it("returns null for minimum length username (2 chars)", () => {
      expect(validateUsername("ab")).toBeNull();
    });
  });

  describe("empty input", () => {
    it("rejects empty string", () => {
      expect(validateUsername("")).toBe("Username is required");
    });

    it("rejects whitespace-only input", () => {
      expect(validateUsername("   ")).toBe("Username is required");
    });
  });

  describe("length requirement", () => {
    it("rejects single character username", () => {
      expect(validateUsername("a")).toBe(
        "Username must be at least 2 characters",
      );
    });
  });
});
