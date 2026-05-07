import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    include: ["__tests__/**/*.test.ts"],
    coverage: {
      provider: "istanbul",
      reporter: ["text", "lcov"],
      exclude: [
        "**/__tests__/**",
        "**/index.ts",
      ],
      thresholds: {
        statements: 95,
        branches: 90,
        functions: 95,
        lines: 95,
      },
    },
  },
});
