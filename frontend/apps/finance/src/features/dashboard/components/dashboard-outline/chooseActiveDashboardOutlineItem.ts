import type { DashboardOutlineItem } from "./types";

interface FlattenedOutlineItem {
  id: string;
  depth: number;
  order: number;
}

export function chooseActiveDashboardOutlineItem(
  items: DashboardOutlineItem[],
  activeIds: Set<string>,
): string | null {
  const activeItems = flattenOutlineItems(items).filter((item) => activeIds.has(item.id));
  if (activeItems.length === 0) return null;

  activeItems.sort((left, right) => {
    if (left.depth !== right.depth) return right.depth - left.depth;
    return left.order - right.order;
  });

  return activeItems[0].id;
}

function flattenOutlineItems(items: DashboardOutlineItem[]): FlattenedOutlineItem[] {
  const flattenedItems: FlattenedOutlineItem[] = [];
  let order = 0;

  function visit(item: DashboardOutlineItem, depth: number) {
    flattenedItems.push({ id: item.id, depth, order });
    order += 1;

    for (const child of item.children) {
      visit(child, depth + 1);
    }
  }

  for (const item of items) {
    visit(item, 0);
  }

  return flattenedItems;
}
