/**
 * Validates that E/D/S (Essentials/Desires/Savings) percentages are valid.
 *
 * Rules:
 * - All values must be non-negative
 * - Values must sum to exactly 100
 *
 * Returns an error message string if invalid, or null if valid.
 */
export function validateEDSSplit(
  essentials: number,
  desires: number,
  savings: number,
): string | null {
  if (essentials < 0 || desires < 0 || savings < 0) {
    return "Percentages must be non-negative";
  }

  const total = essentials + desires + savings;
  if (total !== 100) {
    return `Percentages must sum to 100% (currently ${total}%)`;
  }

  return null;
}

/**
 * Validates password strength.
 *
 * Rules:
 * - Minimum 8 characters
 * - At least one uppercase letter
 * - At least one lowercase letter
 * - At least one digit
 *
 * Returns an error message string if invalid, or null if valid.
 */
export function validatePassword(password: string): string | null {
  if (password.length < 8) {
    return "Password must be at least 8 characters";
  }
  if (!/[A-Z]/.test(password)) {
    return "Password must contain at least one uppercase letter";
  }
  if (!/[a-z]/.test(password)) {
    return "Password must contain at least one lowercase letter";
  }
  if (!/[0-9]/.test(password)) {
    return "Password must contain at least one digit";
  }
  return null;
}

/**
 * Validates email format.
 *
 * Rules:
 * - Must not be empty/whitespace
 * - Must match basic email pattern (user@domain.tld)
 *
 * Returns an error message string if invalid, or null if valid.
 */
export function validateEmail(email: string): string | null {
  if (!email.trim()) {
    return "Email is required";
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    return "Please enter a valid email address";
  }
  return null;
}

/**
 * Validates username.
 *
 * Rules:
 * - Must not be empty/whitespace
 * - Must be at least 2 characters
 *
 * Returns an error message string if invalid, or null if valid.
 */
export function validateUsername(username: string): string | null {
  if (!username.trim()) {
    return "Username is required";
  }
  if (username.trim().length < 2) {
    return "Username must be at least 2 characters";
  }
  return null;
}
