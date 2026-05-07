import { defineConfig } from "vitest/config";
import path from "path";

export default defineConfig({
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./app/__tests__/setup.ts"],
    include: ["app/__tests__/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "istanbul",
      reporter: ["text", "lcov"],
      exclude: [
        "**/__tests__/**",
        "**/types.ts",
        "**/types/**",
        "**/vite-env.d.ts",
        "**/main.tsx",
        "**/index.ts",
        "**/setup.*",
      ],
      thresholds: {
        statements: 71,
        branches: 66,
        functions: 63,
        lines: 71,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./app"),
    },
  },
});
