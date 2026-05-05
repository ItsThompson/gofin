import config from "@gofin/config/eslint";

export default [
  ...config,
  {
    files: ["src/components/**/*.tsx"],
    rules: {
      // UI library exports variant helpers alongside components (shadcn/ui pattern).
      // Fast refresh only applies to app code, not shared packages.
      "react-refresh/only-export-components": "off",
    },
  },
];
