import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

export default defineConfig(({ isSsrBuild }) => ({
  build: {
    rollupOptions: isSsrBuild
      ? {
          input: "./server/app.ts",
        }
      : undefined,
  },
  ssr: {
    // Bundle tree-shakeable packages into SSR output so only imported symbols
    // are included. This eliminates the need for lucide-react (39MB) in
    // production node_modules: only the ~12 icons actually used get bundled.
    noExternal: ["lucide-react"],
  },
  plugins: [tailwindcss(), reactRouter()],
  resolve: {
    alias: {
      "@": new URL("./app", import.meta.url).pathname,
    },
  },
}));
