import { describe, it, expect } from "vitest";
import { formatFileSize } from "../utils/formatFileSize";

describe("formatFileSize", () => {
  it("formats 0 bytes", () => {
    expect(formatFileSize(0)).toBe("0 B");
  });

  it("formats bytes below 1 KB", () => {
    expect(formatFileSize(512)).toBe("512 B");
  });

  it("formats exact kilobytes", () => {
    expect(formatFileSize(1024)).toBe("1 KB");
  });

  it("formats fractional kilobytes", () => {
    expect(formatFileSize(1536)).toBe("1.5 KB");
  });

  it("formats 24 KB", () => {
    expect(formatFileSize(24576)).toBe("24 KB");
  });

  it("formats megabytes", () => {
    expect(formatFileSize(1048576)).toBe("1 MB");
  });

  it("formats fractional megabytes", () => {
    expect(formatFileSize(1258291)).toBe("1.2 MB");
  });

  it("formats gigabytes", () => {
    expect(formatFileSize(1073741824)).toBe("1 GB");
  });
});
