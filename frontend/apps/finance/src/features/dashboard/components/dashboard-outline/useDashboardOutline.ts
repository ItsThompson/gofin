import { useCallback, useEffect, useRef, useState } from "react";
import type { RefObject } from "react";
import { chooseActiveDashboardOutlineItem } from "./chooseActiveDashboardOutlineItem";
import {
  collectDashboardOutline,
  toDashboardOutlineItems,
} from "./collectDashboardOutline";
import type {
  DashboardOutlineElement,
  DashboardOutlineItem,
  DashboardOutlineState,
} from "./types";

const OBSERVED_ATTRIBUTES = [
  "id",
  "data-outline-title",
  "class",
  "style",
  "hidden",
  "aria-hidden",
];

export function useDashboardOutline(rootRef: RefObject<HTMLElement | null>): DashboardOutlineState {
  const [state, setState] = useState<DashboardOutlineState>({ items: [], activeId: null });
  const outlineElementsRef = useRef<DashboardOutlineElement[]>([]);
  const intersectingIdsRef = useRef<Set<string>>(new Set());

  const recollect = useCallback(() => {
    const root = rootRef.current;
    if (!root) {
      outlineElementsRef.current = [];
      intersectingIdsRef.current = new Set();
      setState({ items: [], activeId: null });
      return [];
    }

    const outlineElements = collectDashboardOutline(root);
    const items = toDashboardOutlineItems(outlineElements);
    outlineElementsRef.current = outlineElements;
    intersectingIdsRef.current = new Set(
      Array.from(intersectingIdsRef.current).filter((id) => hasOutlineItem(items, id)),
    );
    const activeId = chooseActiveDashboardOutlineItem(items, intersectingIdsRef.current);
    setState({ items, activeId });

    return flattenOutlineElements(outlineElements).map((item) => item.element);
  }, [rootRef]);

  useEffect(() => {
    const root = rootRef.current;
    if (!root) return;

    recollect();

    const mutationObserver = new MutationObserver(() => {
      recollect();
    });

    mutationObserver.observe(root, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: OBSERVED_ATTRIBUTES,
    });

    return () => {
      mutationObserver.disconnect();
    };
  }, [recollect, rootRef]);

  useEffect(() => {
    const observedElements = flattenOutlineElements(outlineElementsRef.current).map((item) => item.element);
    if (observedElements.length === 0) return;

    const intersectionObserver = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        const target = entry.target;
        if (!(target instanceof HTMLElement)) continue;

        if (entry.isIntersecting) {
          intersectingIdsRef.current.add(target.id);
          continue;
        }

        intersectingIdsRef.current.delete(target.id);
      }

      setState((previousState) => {
        const activeId = chooseActiveDashboardOutlineItem(
          previousState.items,
          intersectingIdsRef.current,
        );
        return { ...previousState, activeId };
      });
    }, { threshold: 0.1 });

    for (const element of observedElements) {
      intersectionObserver.observe(element);
    }

    return () => {
      intersectionObserver.disconnect();
    };
  }, [state.items]);

  return state;
}

function flattenOutlineElements(items: DashboardOutlineElement[]): DashboardOutlineElement[] {
  return items.flatMap((item) => [item, ...flattenOutlineElements(item.children)]);
}

function hasOutlineItem(items: DashboardOutlineItem[], id: string): boolean {
  return items.some((item) => item.id === id || hasOutlineItem(item.children, id));
}
