interface LandingFooterProps {
  brand: string;
  tagline: string;
}

/**
 * Marketing footer landmark: a three-column attribution row (brand left,
 * tagline center, "A t-industri.es project" right), stacked on mobile and
 * spread on md+. The attribution wraps t-industri.es in an external link. No
 * copyright line. Keeps the top border; text uses brand tokens.
 */
export function LandingFooter({ brand, tagline }: LandingFooterProps) {
  return (
    <footer className="border-t">
      <div className="mx-auto flex max-w-7xl flex-col items-center gap-2 px-4 py-10 text-center md:flex-row md:justify-between md:text-left">
        <span className="text-lg font-bold text-foreground">{brand}</span>
        <p className="text-sm text-muted-foreground">{tagline}</p>
        <p className="text-sm text-muted-foreground">
          A{" "}
          <a
            href="https://t-industri.es/"
            target="_blank"
            rel="noopener noreferrer"
            className="underline underline-offset-4 hover:text-foreground"
          >
            t-industri.es
          </a>{" "}
          project
        </p>
      </div>
    </footer>
  );
}
