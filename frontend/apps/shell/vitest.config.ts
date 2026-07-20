import { defineConfig } from "vitest/config";
import path from "path";

export default defineConfig({
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./app/__tests__/setup.ts"],
    include: ["app/__tests__/**/*.test.{ts,tsx}", "app/features/**/__tests__/**/*.test.{ts,tsx}", "server/**/__tests__/**/*.test.{ts,tsx}", "mocks/**/__tests__/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "istanbul",
      reporter: ["text", "lcov"],
      exclude: [
        "**/__tests__/**",
        "**/mocks/**",
        "**/types.ts",
        "**/types/**",
        "**/vite-env.d.ts",
        "**/main.tsx",
        "**/index.ts",
        "**/setup.*",
      ],
      thresholds: {
        statements: 90,
        branches: 85,
        functions: 89,
        lines: 90,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./app"),
    },
  },
});
