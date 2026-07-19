import type { LandingContent } from "./types";

/**
 * The single source of all landing-page copy, links, and section data.
 *
 * Every string here is placeholder GoFin positioning built around the app's
 * essentials / desires / savings model and is expected to be refined by the
 * product owner: refinement is a single-file edit that never touches JSX. The
 * `href` values (`/register`, `/login`) are real routes, not placeholder copy.
 */
export const landingContent: LandingContent = {
  brand: "GoFin",
  tagline: "Know where your money goes.",
  meta: {
    title: "GoFin: know where your money goes",
    description:
      "GoFin sorts every expense into essentials, desires, and savings, so you can see your spending clearly and decide with intention.",
  },
  login: { label: "Log in", href: "/login" },
  hero: {
    heading: "Know exactly where your money goes.",
    subheading:
      "GoFin sorts every expense into essentials, desires, and savings, so you can see your spending clearly and decide with intention.",
    primaryCta: { label: "Get started", href: "/register" },
    ctaFootnote: "Free to start. Set up your first budget in minutes.",
    visualAlt:
      "A GoFin spending breakdown split into essentials, desires, and savings.",
  },
  howItWorks: {
    heading: "How it works",
    steps: [
      {
        ordinal: "01",
        icon: "receipt",
        title: "Log your expenses",
        body: "Add an expense in seconds from any device, or bring in a batch at once.",
      },
      {
        ordinal: "02",
        icon: "pieChart",
        title: "See the breakdown",
        body: "GoFin groups every expense into essentials, desires, and savings so the picture is instantly clear.",
      },
      {
        ordinal: "03",
        icon: "target",
        title: "Spend with intention",
        body: "Spot where money leaks, set targets, and steer each category toward your goals.",
      },
    ],
  },
  valueProp: {
    quote: "Every dollar has a job: essential, desired, or saved.",
    body: "GoFin frames each expense against a simple three-way split, so you always know whether a purchase moves you toward your goals or away from them.",
    footnote:
      "Built around intentional budgeting: clarity first, discipline follows.",
  },
  dualMode: {
    heading: "One view, built for how you actually spend",
    columns: [
      {
        icon: "wallet",
        title: "For everyday tracking",
        body: "Log expenses on the go and always see what's left in each category this month.",
      },
      {
        icon: "lineChart",
        title: "For the bigger picture",
        body: "Review your history and trends to understand your habits and plan ahead with confidence.",
      },
    ],
  },
  faq: {
    heading: "Questions",
    items: [
      {
        question: "Is GoFin free to start?",
        answer:
          "Yes. Create an account and set up your first budget without paying anything.",
      },
      {
        question: "How are my expenses categorized?",
        answer:
          "Every expense is sorted into essentials, desires, or savings, the three buckets GoFin uses to keep spending clear.",
      },
      {
        question: "Can I use GoFin on my phone?",
        answer:
          "Yes. GoFin runs in the browser and is built to work on phones, tablets, and laptops.",
      },
    ],
  },
  finalCta: {
    heading: "Start spending with intention.",
    primaryCta: { label: "Get started", href: "/register" },
  },
};
