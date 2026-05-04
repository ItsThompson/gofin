import "react-router";
import { createRequestHandler } from "@react-router/express";
import express from "express";
import { createProxyMiddleware } from "http-proxy-middleware";
import path from "path";
import { fileURLToPath } from "url";

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
    cookieDomainRewrite: "",
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

// SSR: React Router handles all non-API routes
app.use(
  createRequestHandler({
    build: () => import("virtual:react-router/server-build"),
  }),
);
