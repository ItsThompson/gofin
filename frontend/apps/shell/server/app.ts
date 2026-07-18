import "react-router";
import { createRequestHandler } from "@react-router/express";
import express from "express";
import { createProxyMiddleware } from "http-proxy-middleware";
import path from "path";
import { fileURLToPath } from "url";
import { createHealthzHandler } from "./healthz";

const API_GATEWAY_URL =
  process.env.API_GATEWAY_URL || "http://localhost:8080";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export const app = express();

// Reverse proxy: /api/* → API Gateway
// Forwards cookies and sets appropriate headers for downstream services.
app.use(
  createProxyMiddleware({
    target: API_GATEWAY_URL,
    pathFilter: "/api",
    changeOrigin: true,
    on: {
      proxyReq: (proxyReq, req) => {
        // Forward the original host for logging/audit
        const forwardedFor =
          req.headers["x-forwarded-for"] || req.socket.remoteAddress;
        proxyReq.setHeader("X-Forwarded-For", String(forwardedFor));
      },
    },
  }),
);

// Serve remote static assets from a single origin.
// Finance and admin remotes build to dist/; the shell serves their assets
// so the browser fetches everything from the same origin.
const remotesBase = path.resolve(__dirname, "../../");

app.use(
  "/remotes/admin",
  express.static(path.join(remotesBase, "admin/dist"), {
    maxAge: "1h",
    immutable: false,
  }),
);

app.use(
  "/remotes/finance",
  express.static(path.join(remotesBase, "finance/dist"), {
    maxAge: "1h",
    immutable: false,
  }),
);

// Public liveness endpoint for the external health-check cron. Mirrors the
// gateway's /readyz aggregate (200 when all backend services are healthy, else
// 503) and never hangs on a slow gateway. Mounted after the /api proxy and
// before the SSR catch-all so it is not proxied or swallowed by SSR.
app.get(
  "/healthz",
  createHealthzHandler({
    gatewayUrl: API_GATEWAY_URL,
    fetchFn: fetch,
    timeoutMs: 5000,
  }),
);

// SSR: React Router handles all non-API routes
app.use(
  createRequestHandler({
    build: () => import("virtual:react-router/server-build"),
  }),
);
