interface LandingFooterProps {
  brand: string;
  tagline: string;
}

/**
 * Marketing footer landmark: the brand wordmark, the tagline, and a copyright
 * line. Thin presentational component with no logic; text and the top border
 * come from brand tokens. No third-party attribution is rendered (spec §06).
 */
export function LandingFooter({ brand, tagline }: LandingFooterProps) {
  const year = new Date().getFullYear();

  return (
    <footer className="border-t">
      <div className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-10">
        <span className="text-lg font-bold text-foreground">{brand}</span>
        <p className="text-sm text-muted-foreground">{tagline}</p>
        <p className="text-sm text-muted-foreground">
          © {year} {brand}. All rights reserved.
        </p>
      </div>
    </footer>
  );
}
