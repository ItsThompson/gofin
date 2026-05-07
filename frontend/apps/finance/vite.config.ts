import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { federation } from "@module-federation/vite";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    federation({
      name: "finance",
      filename: "remoteEntry.js",
      exposes: {
        "./DashboardPage": "./src/pages/DashboardPage.tsx",
        "./SettingsPage": "./src/features/settings/index.ts",
        "./NewExpensePage": "./src/pages/NewExpensePage.tsx",
        "./ExpenseLogPage": "./src/pages/ExpenseLogPage.tsx",
        "./routes": "./src/routes.ts",
      },
      shared: {
        react: { singleton: true, requiredVersion: "^19" },
        "react-dom": { singleton: true, requiredVersion: "^19" },
        "react-router": { singleton: true, requiredVersion: "^7" },
        zustand: { singleton: true, requiredVersion: "^5" },
      },
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    target: "esnext",
    minify: false,
  },
});
