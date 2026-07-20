/**
 * Module Federation host configuration for the shell app.
 *
 * This config declares the finance and admin remotes plus the shared
 * singletons (react, react-dom, react-router, zustand) that define the
 * Module Federation boundary for the shell host.
 */
export const moduleFederationConfig = {
  name: "shell",
  remotes: {
    finance: {
      type: "module" as const,
      name: "finance",
      entry: "/remotes/finance/remoteEntry.js",
    },
    admin: {
      type: "module" as const,
      name: "admin",
      entry: "/remotes/admin/remoteEntry.js",
    },
  },
  shared: {
    react: { singleton: true, requiredVersion: "^19" },
    "react-dom": { singleton: true, requiredVersion: "^19" },
    "react-router": { singleton: true, requiredVersion: "^7" },
    zustand: { singleton: true, requiredVersion: "^5" },
  },
};
