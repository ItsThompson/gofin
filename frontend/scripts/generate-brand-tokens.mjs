#!/usr/bin/env node

/**
 * Generates brand-tokens.css from the shared tokens/brand.json file.
 *
 * This ensures the frontend CSS theme and the Go email templates both consume
 * the same source of truth for brand colors.
 *
 * Usage: node scripts/generate-brand-tokens.mjs
 */

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const tokensPath = resolve(__dirname, "../../tokens/brand.json");
const outputPath = resolve(
  __dirname,
  "../packages/ui/src/styles/brand-tokens.css",
);

const tokens = JSON.parse(readFileSync(tokensPath, "utf-8"));

/**
 * Converts camelCase to kebab-case.
 * e.g., "primaryForeground" -> "primary-foreground"
 */
function toKebab(str) {
  return str.replace(/([A-Z])/g, "-$1").toLowerCase();
}

/**
 * Generates CSS custom property declarations from a color map.
 */
function generateVars(colors, indent = "    ") {
  return Object.entries(colors)
    .map(([key, value]) => `${indent}--brand-${toKebab(key)}: ${value};`)
    .join("\n");
}

const css = `/* Auto-generated from tokens/brand.json — do not edit manually. */
/* Regenerate with: npm run generate:tokens (from frontend/) */

:root {
${generateVars(tokens.colors)}
}

.dark {
${generateVars(tokens.colorsDark)}
}
`;

writeFileSync(outputPath, css);
console.log(`✓ Generated brand-tokens.css from tokens/brand.json`);
