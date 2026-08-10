/**
 * `instrument.server.mjs` is plain JavaScript that exports nothing: importing it
 * performs the server `Sentry.init`. This declaration exists so a TypeScript
 * importer resolves the specifier under the strict base, which leaves `allowJs`
 * unset for the whole app.
 */
export {};
