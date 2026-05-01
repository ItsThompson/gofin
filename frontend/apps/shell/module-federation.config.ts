/**
 * Module Federation host configuration for the shell app.
 *
 * The shell is the MF host that loads finance and admin remotes at runtime.
 * Remote loading is wired in ticket 8: for now, this config declares the
 * remotes and shared singletons so the MF boundary is established.
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
    "@gofin/types": { singleton: true },
    "@gofin/ui": { singleton: true },
  },
};
