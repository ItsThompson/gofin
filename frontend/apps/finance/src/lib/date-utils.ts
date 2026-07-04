/**
 * Formats a `Date` into a local calendar date string in `YYYY-MM-DD` form.
 *
 * Uses the local timezone components (`getFullYear`/`getMonth`/`getDate`)
 * rather than `toISOString`, which returns the UTC date and can be off by a
 * day near midnight depending on the runtime timezone. The returned value is
 * therefore the local calendar date, suitable for date inputs and expense
 * dates. Defaults to the current date/time.
 */
export function toLocalISODate(date: Date = new Date()): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}
