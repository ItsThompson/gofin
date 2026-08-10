import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

export default defineConfig(({ isSsrBuild }) => ({
  build: {
    // "hidden" emits .map files with no trailing sourceMappingURL comment, so a
    // browser never requests one. The bundle is minified, so without maps every
    // Sentry stack points into single-letter chunk code. The maps are uploaded
    // to Sentry from the Docker builder stage and deleted from the runtime
    // image, so they never reach a client.
    sourcemap: "hidden",
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
