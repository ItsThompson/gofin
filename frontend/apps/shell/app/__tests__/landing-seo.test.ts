import { describe, it, expect } from "vitest";
import { meta } from "@/routes/home";
import { landingContent } from "@/features/marketing";

// The `/` route's SEO metadata is a static route-module export. React Router
// invokes it during SSR to populate <Meta /> in root.tsx's <head> (see §04), so
// asserting the export is the sanctioned way to verify the server HTML carries
// the title/description (US-SEO-01). `meta` is a plain function with no React or
// browser dependency: it produces its values without a rendered component or
// client auth state, which is what makes them present in the server-rendered
// HTML rather than injected only after hydration.
describe("/ route SEO metadata (home.tsx meta export)", () => {
  // `meta` ignores its MetaArgs; the empty stand-in is the single call site.
  const descriptors = meta({} as Parameters<typeof meta>[0]);

  it("derives the document <title> from the content model", () => {
    expect(descriptors).toContainEqual({ title: landingContent.meta.title });
  });

  it('derives the <meta name="description"> from the content model', () => {
    expect(descriptors).toContainEqual({
      name: "description",
      content: landingContent.meta.description,
    });
  });
});
