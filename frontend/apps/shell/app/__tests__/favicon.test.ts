import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { createRoutesStub } from "react-router";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

import { Layout, links } from "../root";

const publicDirectory = path.resolve(import.meta.dirname, "../../public");

describe("shared favicon package", () => {
  it("declares browser, Apple, and manifest links from the root route", () => {
    expect(links()).toEqual([
      {
        rel: "icon",
        href: "/favicon.ico",
        sizes: "16x16 32x32 48x48",
      },
      {
        rel: "icon",
        type: "image/png",
        sizes: "16x16",
        href: "/favicon/favicon-16x16.png",
      },
      {
        rel: "icon",
        type: "image/png",
        sizes: "32x32",
        href: "/favicon/favicon-32x32.png",
      },
      {
        rel: "icon",
        type: "image/svg+xml",
        sizes: "any",
        href: "/favicon/favicon.svg",
      },
      {
        rel: "apple-touch-icon",
        sizes: "180x180",
        href: "/favicon/apple-touch-icon.png",
      },
      { rel: "manifest", href: "/site.webmanifest" },
    ]);
  });

  it("includes every asset referenced by the root links", () => {
    for (const descriptor of links()) {
      expect(descriptor).toHaveProperty("href");
      if ("href" in descriptor && descriptor.href) {
        expect(existsSync(path.join(publicDirectory, descriptor.href))).toBe(true);
      }
    }
  });

  it("provides Android icons without changing the browser launch experience", () => {
    const manifest = JSON.parse(
      readFileSync(path.join(publicDirectory, "site.webmanifest"), "utf8"),
    );
    expect(manifest).toEqual({
      name: "GoFin",
      short_name: "GoFin",
      icons: [
        {
          src: "/favicon/android-chrome-192x192.png",
          sizes: "192x192",
          type: "image/png",
          purpose: "any",
        },
        {
          src: "/favicon/android-chrome-512x512.png",
          sizes: "512x512",
          type: "image/png",
          purpose: "any",
        },
      ],
      theme_color: "#ffffff",
      background_color: "#ffffff",
      display: "browser",
    });
    for (const icon of manifest.icons) {
      expect(existsSync(path.join(publicDirectory, icon.src))).toBe(true);
    }
  });

  it("includes Windows tile metadata in the shared document head", () => {
    const RoutesStub = createRoutesStub([
      {
        path: "/",
        links,
        Component: () => createElement(Layout, { children: createElement("main") }),
      },
    ]);
    const html = renderToString(createElement(RoutesStub));
    const document = new DOMParser().parseFromString(html, "text/html");
    expect(
      document.head.querySelector('meta[name="msapplication-config"]')?.getAttribute("content"),
    ).toBe("/browserconfig.xml");
    expect(
      document.head.querySelector('meta[name="msapplication-TileColor"]')?.getAttribute("content"),
    ).toBe("#ffffff");
    expect(
      document.head.querySelector('link[href="/favicon/favicon.svg"]')?.getAttribute("rel"),
    ).toBe("icon");
  });

  it("provides a Windows tile with a white background", () => {
    const xml = new DOMParser().parseFromString(
      readFileSync(path.join(publicDirectory, "browserconfig.xml"), "utf8"),
      "application/xml",
    );
    expect(xml.querySelector("parsererror")).toBeNull();
    const image = xml.querySelector("square150x150logo")?.getAttribute("src");
    expect(image).toBe("/favicon/mstile-150x150.png");
    expect(xml.querySelector("TileColor")?.textContent).toBe("#ffffff");
    expect(existsSync(path.join(publicDirectory, "/favicon/mstile-150x150.png"))).toBe(true);
  });
});
