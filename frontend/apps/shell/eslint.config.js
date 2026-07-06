import config from "@gofin/config/eslint";

export default [
  ...config,
  {
    // React Router route modules export route metadata (`handle`, and may export
    // `loader`/`meta`/`action`) alongside their component. That is the
    // framework's convention, and HMR for route modules is handled by the React
    // Router dev plugin, so the generic react-refresh component-only rule does
    // not apply to these files.
    files: ["app/routes/**/*.tsx"],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },
];
