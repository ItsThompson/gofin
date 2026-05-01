/** Client-side validation matching server rules (8+ chars, 1 upper, 1 lower, 1 digit). */
export function validatePassword(password: string): string | null {
  if (password.length < 8) {
    return "Password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit";
  }

  const hasUpper = /[A-Z]/.test(password);
  const hasLower = /[a-z]/.test(password);
  const hasDigit = /[0-9]/.test(password);

  if (!hasUpper || !hasLower || !hasDigit) {
    return "Password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit";
  }

  return null;
}

export function validateEmail(email: string): string | null {
  if (!email.trim()) {
    return "Email is required";
  }
  // Basic email format check matching typical server validation
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    return "Please enter a valid email address";
  }
  return null;
}

export function validateUsername(username: string): string | null {
  if (!username.trim()) {
    return "Username is required";
  }
  if (username.trim().length < 2) {
    return "Username must be at least 2 characters";
  }
  return null;
}
