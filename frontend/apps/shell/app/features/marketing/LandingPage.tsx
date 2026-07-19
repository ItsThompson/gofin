import { useLandingRedirect } from "./hooks/useLandingRedirect";
import { landingContent } from "./content";
import { LandingHeader } from "./components/LandingHeader";
import { HeroSection } from "./components/HeroSection";

/**
 * Marketing landing page orchestrator. Runs the authenticated-visitor redirect,
 * then composes the marketing sections from the content model.
 *
 * This slice renders the header and hero; later section tickets append the
 * remaining sections and the footer into <main>.
 */
export function LandingPage() {
  useLandingRedirect();
  const content = landingContent;

  return (
    <div className="min-h-screen bg-background text-foreground">
      <LandingHeader brand={content.brand} login={content.login} />
      <main>
        <HeroSection {...content.hero} />
      </main>
    </div>
  );
}
