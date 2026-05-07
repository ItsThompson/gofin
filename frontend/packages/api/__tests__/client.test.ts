import { describe, it, expect } from "vitest";
import { ApiRequestError } from "../src/client";

describe("ApiRequestError", () => {
  it("constructs with status, code, and message from ApiError", () => {
    const error = new ApiRequestError(400, {
      code: "VALIDATION_ERROR",
      message: "Invalid email format",
    });

    expect(error.status).toBe(400);
    expect(error.code).toBe("VALIDATION_ERROR");
    expect(error.message).toBe("Invalid email format");
    expect(error.fields).toBeUndefined();
  });

  it("includes field-level errors when present", () => {
    const error = new ApiRequestError(422, {
      code: "VALIDATION_ERROR",
      message: "Validation failed",
      fields: {
        email: "Must be a valid email",
        password: "Too short",
      },
    });

    expect(error.status).toBe(422);
    expect(error.code).toBe("VALIDATION_ERROR");
    expect(error.fields).toEqual({
      email: "Must be a valid email",
      password: "Too short",
    });
  });

  it("has name set to ApiRequestError", () => {
    const error = new ApiRequestError(500, {
      code: "INTERNAL_ERROR",
      message: "Something went wrong",
    });

    expect(error.name).toBe("ApiRequestError");
  });

  it("is an instance of Error", () => {
    const error = new ApiRequestError(404, {
      code: "NOT_FOUND",
      message: "Resource not found",
    });

    expect(error).toBeInstanceOf(Error);
    expect(error).toBeInstanceOf(ApiRequestError);
  });

  it("preserves stack trace", () => {
    const error = new ApiRequestError(403, {
      code: "FORBIDDEN",
      message: "Access denied",
    });

    expect(error.stack).toBeDefined();
    expect(error.stack).toContain("ApiRequestError");
  });

  it("handles empty fields object", () => {
    const error = new ApiRequestError(400, {
      code: "VALIDATION_ERROR",
      message: "Invalid",
      fields: {},
    });

    expect(error.fields).toEqual({});
  });
});
