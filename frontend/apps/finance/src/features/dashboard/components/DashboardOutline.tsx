import type { ReactNode } from "react";
import {
  useDashboardOutline,
  type DashboardOutlineItem,
  type DashboardOutlineProps,
} from "./dashboard-outline";

export function DashboardOutline({ rootRef }: DashboardOutlineProps) {
  const { items, activeId } = useDashboardOutline(rootRef);

  if (items.length === 0) return null;

  return (
    <nav
      aria-label="Dashboard sections"
      className="hidden xl:block fixed top-20 right-8 w-48"
    >
      <ul className="space-y-1 text-xs text-muted-foreground">
        {renderOutlineItems(items, activeId, false)}
      </ul>
    </nav>
  );
}

function renderOutlineItems(
  items: DashboardOutlineItem[],
  activeId: string | null,
  isNested: boolean,
): ReactNode {
  return items.map((item) => (
    <li key={item.id}>
      <a
        href={`#${item.id}`}
        aria-current={activeId === item.id ? "location" : undefined}
        className={getLinkClassName(activeId === item.id, isNested)}
      >
        {item.title}
      </a>
      {item.children.length > 0 && (
        <ul className="ml-3 border-l border-border pl-2 space-y-0.5">
          {renderOutlineItems(item.children, activeId, true)}
        </ul>
      )}
    </li>
  ));
}

function getLinkClassName(isActive: boolean, isNested: boolean): string {
  const sizeClassName = isNested ? "py-0.5 text-[11px]" : "py-1";
  const activeClassName = isActive ? "text-foreground font-medium" : "hover:text-foreground";

  return `block ${sizeClassName} ${activeClassName} transition-colors`;
}
