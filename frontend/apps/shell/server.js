// First statement, above express, because this is the process entry: the init
// must complete before express loads, before the SSR bundle is imported and
// before any request. See instrument.server.mjs.
import "./instrument.server.mjs";
import express from "express";

const BUILD_PATH = "./build/server/index.js";
const DEVELOPMENT = process.env.NODE_ENV === "development";
const PORT = Number.parseInt(process.env.PORT || "3000");

const app = express();
app.disable("x-powered-by");

if (DEVELOPMENT) {
  console.log("Starting development server");
  const viteDevServer = await import("vite").then((vite) =>
    vite.createServer({
      server: { middlewareMode: true },
    }),
  );
  app.use(viteDevServer.middlewares);
  app.use(async (req, res, next) => {
    try {
      const source = await viteDevServer.ssrLoadModule("./server/app.ts");
      return await source.app(req, res, next);
    } catch (error) {
      if (typeof error === "object" && error instanceof Error) {
        viteDevServer.ssrFixStacktrace(error);
      }
      next(error);
    }
  });
} else {
  console.log("Starting production server");
  app.use(
    "/assets",
    express.static("build/client/assets", { immutable: true, maxAge: "1y" }),
  );
  app.use(express.static("build/client", { maxAge: "1h" }));
  const build = await import(BUILD_PATH);
  app.use(build.app);
  // Express 5 answers 500 through its default handler, which records nothing.
  // The reporter and the body come off the bundle namespace because this file is
  // outside the bundle: a workspace specifier here does not resolve in the
  // runner image, and a dynamic import would fail at error time, which is the
  // one moment the middleware exists for.
  app.use((error, _req, res, next) => {
    build.reportServerError(error);
    // The SSR response resolves as soon as the shell is ready and the rest is
    // piped, so an error can arrive after the status line is on the wire.
    if (res.headersSent) {
      next(error);
      return;
    }
    res.status(500).json(build.serverErrorBody);
  });
}

app.listen(PORT, () => {
  console.log(`Server is running on http://localhost:${PORT}`);
});
