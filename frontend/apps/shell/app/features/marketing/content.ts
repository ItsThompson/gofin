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
        body: "Log, categorize, and tag your expenses in seconds from any device.",
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
      "Seeing your expenses clearly makes spending with intention stop feeling like work.",
  },
  threeWaySplit: {
    heading: "Every expense fits one of three buckets",
    intro:
      "GoFin sorts your spending into three simple categories, so every purchase has a clear place.",
    buckets: [
      {
        accent: "essentials",
        icon: "house",
        title: "Essentials",
        body: "The non-negotiables: rent, groceries, bills. The spending that keeps life running.",
      },
      {
        accent: "desires",
        icon: "sparkles",
        title: "Desires",
        body: "The wants: eating out, subscriptions, the occasional impulse buy. Enjoyable, as long as it's intentional.",
      },
      {
        accent: "savings",
        icon: "piggyBank",
        title: "Savings",
        body: "Money you set aside first, so your goals aren't just whatever's left over.",
      },
    ],
  },
  featureShowcase: {
    heading: "Built to keep you intentional",
    features: [
      {
        icon: "gauge",
        title: "Know if you're on track",
        body: "GoFin paces your budget across the month, so you can see whether you're ahead or overspending before it's too late.",
      },
      {
        icon: "calendarClock",
        title: "Big purchase? Spread it out",
        body: "Log a large expense once and spread it across several months, so one purchase doesn't blow up a single month's budget.",
      },
      {
        icon: "lineChart",
        title: "Watch your habits change",
        body: "Track spending month over month to see where your money really goes, and how your choices shift over time.",
      },
    ],
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
        question: "How is GoFin different from my banking app?",
        answer:
          "Your bank lists what you spent. GoFin sorts every expense into essentials, desires, and savings, so you can see whether your spending matches your priorities, not just your balance.",
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
