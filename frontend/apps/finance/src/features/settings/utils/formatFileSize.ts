const UNITS = ["B", "KB", "MB", "GB"] as const;

/**
 * Formats a byte count into a human-readable string.
 * Examples: 0 → "0 B", 1024 → "1 KB", 1536 → "1.5 KB", 1048576 → "1 MB"
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return "0 B";

  const exponent = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    UNITS.length - 1,
  );
  const value = bytes / Math.pow(1024, exponent);
  const formatted = value % 1 === 0 ? value.toString() : value.toFixed(1);

  return `${formatted} ${UNITS[exponent]}`;
}
