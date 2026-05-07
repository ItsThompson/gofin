import { defineConfig } from "vitest/config";
import path from "path";

export default defineConfig({
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/__tests__/setup.ts"],
    include: ["src/__tests__/**/*.test.{ts,tsx}"],
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
        statements: 96,
        branches: 94,
        functions: 100,
        lines: 96,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
