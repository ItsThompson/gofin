import type { RefObject } from "react";

export interface DashboardOutlineItem {
  id: string;
  title: string;
  children: DashboardOutlineItem[];
}

export interface DashboardOutlineState {
  items: DashboardOutlineItem[];
  activeId: string | null;
}

export interface DashboardOutlineElement {
  id: string;
  title: string;
  element: HTMLElement;
  children: DashboardOutlineElement[];
}

export interface DashboardOutlineProps {
  rootRef: RefObject<HTMLElement | null>;
}
