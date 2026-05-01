import "react-router";
import { createRequestHandler } from "@react-router/express";
import express from "express";
import { createProxyMiddleware } from "http-proxy-middleware";

const API_GATEWAY_URL =
  process.env.API_GATEWAY_URL || "http://localhost:8080";

export const app = express();

// Reverse proxy: /api/* → API Gateway
// Forwards cookies and sets appropriate headers for downstream services.
app.use(
  "/api",
  createProxyMiddleware({
    target: API_GATEWAY_URL,
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

// SSR: React Router handles all non-API routes
app.use(
  createRequestHandler({
    // @ts-expect-error virtual module provided by react-router vite plugin
    build: () => import("virtual:react-router/server-build"),
  }),
);
