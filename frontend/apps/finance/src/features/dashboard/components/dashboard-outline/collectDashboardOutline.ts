import type { DashboardOutlineElement, DashboardOutlineItem } from "./types";

const OUTLINE_SELECTOR = "[id][data-outline-title]";

export function collectDashboardOutline(root: HTMLElement): DashboardOutlineElement[] {
  const candidates = Array.from(root.querySelectorAll<HTMLElement>(OUTLINE_SELECTOR));
  const outlineElements = candidates.reduce<DashboardOutlineElement[]>((items, element) => {
    if (!isElementVisible(element)) return items;

    const title = element.dataset.outlineTitle?.trim();
    if (!title) return items;

    items.push({
      id: element.id,
      title,
      element,
      children: [],
    });

    return items;
  }, []);

  return buildOutlineTree(outlineElements);
}

export function toDashboardOutlineItems(
  outlineElements: DashboardOutlineElement[],
): DashboardOutlineItem[] {
  return outlineElements.map((item) => ({
    id: item.id,
    title: item.title,
    children: toDashboardOutlineItems(item.children),
  }));
}

function buildOutlineTree(items: DashboardOutlineElement[]): DashboardOutlineElement[] {
  const roots: DashboardOutlineElement[] = [];

  for (const item of items) {
    const parent = findNearestCollectedParent(item, items);

    if (parent) {
      parent.children.push(item);
      continue;
    }

    roots.push(item);
  }

  return roots;
}

function findNearestCollectedParent(
  item: DashboardOutlineElement,
  items: DashboardOutlineElement[],
): DashboardOutlineElement | null {
  const itemIndex = items.indexOf(item);
  let nearestParent: DashboardOutlineElement | null = null;

  for (const candidate of items.slice(0, itemIndex)) {
    if (!candidate.element.contains(item.element)) continue;
    if (nearestParent && !nearestParent.element.contains(candidate.element)) continue;

    nearestParent = candidate;
  }

  return nearestParent;
}

function isElementVisible(element: HTMLElement): boolean {
  if (element.hidden || element.getAttribute("aria-hidden") === "true") return false;

  const style = window.getComputedStyle(element);
  if (style.display === "none" || style.visibility === "hidden") return false;

  const rect = element.getBoundingClientRect();
  if (rect.width > 0 || rect.height > 0) return true;

  return element.offsetParent !== null;
}
