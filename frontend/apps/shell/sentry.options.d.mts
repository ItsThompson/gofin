import type { BrowserOptions } from "@sentry/react-router";

/**
 * Hand-written because `sentry.options.mjs` is plain JavaScript and `apps/shell`
 * type-checks with `allowJs` unset. A declaration file keeps the strict base
 * settings for the rest of the app; `allowJs: true` would change checking for
 * every file.
 */

/** The keys of `SHARED_OPTIONS`, all of which exist on both platforms' options. */
export type SharedOptions = Pick<
  BrowserOptions,
  "environment" | "dataCollection" | "tracesSampleRate" | "ignoreErrors"
> & {
  // `integrations` also accepts a callback form on the SDK's own type; this
  // module always builds an array.
  integrations: Extract<
    NonNullable<BrowserOptions["integrations"]>,
    readonly unknown[]
  >;
};

export declare const SHARED_OPTIONS: SharedOptions;

export interface ClientOptionsArgs {
  dsn: string;
  /** Already prefixed as `gofin-web@<sha>`; the builder does not transform it. */
  release: string;
  /** Supplied by the caller, because this module cannot reach the api package. */
  isNetworkError: (error: unknown) => boolean;
}

/** `BrowserOptions` narrowed on the keys callers and tests read back. */
export interface ClientInitOptions extends BrowserOptions {
  dsn: string;
  release: string;
  initialScope: { tags: Record<string, string> };
  replaysSessionSampleRate: number;
  replaysOnErrorSampleRate: number;
  denyUrls: RegExp[];
  beforeSend: NonNullable<BrowserOptions["beforeSend"]>;
}

export declare function clientOptions(args: ClientOptionsArgs): ClientInitOptions;
