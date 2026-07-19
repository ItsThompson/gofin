/**
 * Content model and prop types for the marketing landing feature.
 *
 * The content model is plain, serializable data: no React nodes and no
 * component references (icons are string keys resolved in icons.ts). This keeps
 * content.ts pure data that downstream sections consume as props.
 */

/** A call-to-action link. Always rendered as a real <Link>/<a href>. */
export interface CtaLink {
  label: string;
  href: string;
}

/** Icon keys the landing page may use. Resolved to lucide components in icons.ts. */
export type LandingIcon =
  | "receipt" // logging expenses
  | "pieChart" // category breakdown
  | "target" // goals / intention
  | "wallet" // everyday tracking
  | "lineChart" // trends / bigger picture
  | "gauge" // pacing: on-track indicator
  | "calendarClock" // spreading a large expense across months
  | "house" // essentials bucket
  | "sparkles" // desires bucket
  | "piggyBank"; // savings bucket

/** Hero section content. */
export interface HeroContent {
  heading: string; // the single <h1>
  subheading: string; // supporting paragraph
  primaryCta: CtaLink; // -> /register
  visualAlt: string; // aria-label for the animated hero scene
}

/** One numbered step in "How it works". */
export interface StepContent {
  ordinal: string; // zero-padded, e.g. "01"
  icon: LandingIcon;
  title: string;
  body: string;
}

/** The accent value-proposition band. */
export interface ValuePropContent {
  quote: string; // large text inside the accent card
  body: string; // supporting paragraph
  footnote: string; // smaller muted line
}

/** One column in the dual-mode split. */
export interface FeatureColumnContent {
  icon: LandingIcon;
  title: string;
  body: string;
}

/** A category accent that maps to an essentials/desires/savings color token. */
export type SplitAccent = "essentials" | "desires" | "savings";

/** One bucket in "The three-way split". */
export interface SplitBucketContent {
  accent: SplitAccent; // maps to the category color token
  icon: LandingIcon;
  title: string;
  body: string;
}

/** One feature in "Feature showcase" (text + icon, no mock UI). */
export interface ShowcaseFeatureContent {
  icon: LandingIcon;
  title: string;
  body: string;
}

/** One FAQ entry. */
export interface FaqItemContent {
  question: string;
  answer: string;
}

/** Page <title> + meta description for SEO. Server-rendered via the route meta. */
export interface LandingMeta {
  title: string; // the document <title> for /
  description: string; // <meta name="description">; kept in sync with hero.subheading
}

/** "How it works" section content. */
export interface HowItWorksContent {
  heading: string;
  steps: StepContent[];
}

/** "The three-way split" section content. */
export interface ThreeWaySplitContent {
  heading: string;
  intro: string;
  buckets: SplitBucketContent[];
}

/** "Feature showcase" section content. */
export interface FeatureShowcaseContent {
  heading: string;
  features: ShowcaseFeatureContent[];
}

/** Dual-mode split section content. */
export interface DualModeContent {
  heading: string;
  columns: FeatureColumnContent[];
}

/** FAQ section content. */
export interface FaqContent {
  heading: string;
  items: FaqItemContent[];
}

/** Closing call-to-action section content. */
export interface FinalCtaContent {
  heading: string;
  primaryCta: CtaLink;
}

/** The single source of all landing copy, links, and section data. */
export interface LandingContent {
  brand: string; // "GoFin"
  tagline: string; // footer tagline
  meta: LandingMeta; // SEO title + description (read by the route `meta`)
  login: CtaLink; // header "Log in" -> /login
  hero: HeroContent;
  howItWorks: HowItWorksContent;
  valueProp: ValuePropContent;
  threeWaySplit: ThreeWaySplitContent;
  featureShowcase: FeatureShowcaseContent;
  dualMode: DualModeContent;
  faq: FaqContent;
  finalCta: FinalCtaContent;
}

/** Props for the landing header (brand wordmark + login link). */
export interface LandingHeaderProps {
  brand: string;
  login: CtaLink;
}

/** Props for the hero placeholder visual. */
export interface HeroVisualProps {
  alt: string;
}
