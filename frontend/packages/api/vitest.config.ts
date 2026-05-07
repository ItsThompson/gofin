import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "jsdom",
    include: ["__tests__/**/*.test.ts"],
    coverage: {
      provider: "istanbul",
      reporter: ["text", "lcov"],
      exclude: [
        "**/__tests__/**",
        "**/index.ts",
      ],
      thresholds: {
        statements: 90,
        branches: 85,
        functions: 90,
        lines: 90,
      },
    },
  },
});
