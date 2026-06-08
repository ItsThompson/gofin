const TOC_ITEMS = [
  {
    label: "Summary",
    href: "#summary",
    children: [
      { label: "Budget Allocations", href: "#budget-allocations" },
      { label: "Spending Pace", href: "#spending-pace" },
      { label: "Historical Comparison", href: "#historical-comparison" },
    ],
  },
  {
    label: "Trends",
    href: "#trends",
    children: [
      { label: "Monthly Spending", href: "#trends" },
      { label: "Category Split", href: "#trends" },
    ],
  },
  {
    label: "Breakdown",
    href: "#breakdown",
    children: [
      { label: "Spending by Tag", href: "#breakdown" },
      { label: "Repeated Expenses", href: "#breakdown" },
    ],
  },
  { label: "Cumulative Spending", href: "#cumulative-spending" },
  { label: "Recent Expenses", href: "#recent-expenses" },
] as const;

interface TocItem {
  label: string;
  href: string;
  children?: readonly { label: string; href: string }[];
}

export function DashboardOutline() {
  return (
    <nav
      aria-label="Dashboard sections"
      className="hidden xl:block fixed top-20 right-8 w-48"
    >
      <ul className="space-y-1 text-xs text-muted-foreground">
        {TOC_ITEMS.map((item) => (
          <TocEntry key={item.label} item={item} />
        ))}
      </ul>
    </nav>
  );
}

function TocEntry({ item }: { item: TocItem }) {
  return (
    <li>
      <a
        href={item.href}
        className="block py-1 hover:text-foreground transition-colors"
      >
        {item.label}
      </a>
      {item.children && item.children.length > 0 && (
        <ul className="ml-3 border-l border-border pl-2 space-y-0.5">
          {item.children.map((child) => (
            <li key={child.label}>
              <a
                href={child.href}
                className="block py-0.5 text-[11px] hover:text-foreground transition-colors"
              >
                {child.label}
              </a>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}
