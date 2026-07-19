/**
 * Derives the single uppercase initial shown in the avatar fallback from a
 * username. Trims surrounding whitespace and returns an empty string for an
 * empty username (the avatar then renders an empty circle rather than throwing).
 */
export function getAvatarInitial(username: string): string {
  return username.trim().charAt(0).toUpperCase();
}
