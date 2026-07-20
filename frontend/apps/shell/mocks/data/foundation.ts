export const now = new Date();
export const currentYear = now.getFullYear();
export const currentMonth = now.getMonth() + 1;
export const daysInMonth = new Date(currentYear, currentMonth, 0).getDate();
export const daysElapsed = now.getDate();

export function uuid(): string {
  return crypto.randomUUID();
}

export function daysAgoISO(days: number): string {
  const date = new Date(now);
  date.setDate(date.getDate() - days);
  return date.toISOString().slice(0, 10);
}
