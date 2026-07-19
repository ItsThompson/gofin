import { useLandingRedirect } from "./hooks/useLandingRedirect";
import { landingContent } from "./content";
import { LandingHeader } from "./components/LandingHeader";
import { HeroSection } from "./components/HeroSection";
import { HowItWorksSection } from "./components/HowItWorksSection";
import { ValuePropSection } from "./components/ValuePropSection";
import { DualModeSection } from "./components/DualModeSection";
import { FaqSection } from "./components/FaqSection";
import { FinalCtaSection } from "./components/FinalCtaSection";
import { LandingFooter } from "./components/LandingFooter";

/**
 * Marketing landing page orchestrator. Runs the authenticated-visitor redirect,
 * then composes the ordered marketing sections from the content model: a single
 * <header>, the section stack inside one <main>, and one <footer> landmark.
 */
export function LandingPage() {
  useLandingRedirect();
  const content = landingContent;

  return (
    <div className="min-h-screen bg-background text-foreground">
      <LandingHeader brand={content.brand} login={content.login} />
      <main>
        <HeroSection {...content.hero} />
        <HowItWorksSection {...content.howItWorks} />
        <ValuePropSection {...content.valueProp} />
        <DualModeSection {...content.dualMode} />
        <FaqSection {...content.faq} />
        <FinalCtaSection {...content.finalCta} />
      </main>
      <LandingFooter brand={content.brand} tagline={content.tagline} />
    </div>
  );
}
