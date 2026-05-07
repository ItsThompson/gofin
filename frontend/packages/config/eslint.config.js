import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist", "build", ".react-router", "**/public/mockServiceWorker.js"] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["**/features/*/components/*"],
              message:
                "Import from the feature barrel (features/name) instead.",
            },
            {
              group: ["**/features/*/hooks/*"],
              message:
                "Import from the feature barrel (features/name) instead.",
            },
            {
              group: ["**/features/*/api"],
              message:
                "Import from the feature barrel (features/name) instead.",
            },
          ],
        },
      ],
    },
  },
);
