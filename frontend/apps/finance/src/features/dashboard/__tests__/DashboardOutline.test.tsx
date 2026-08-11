import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { createRef, type ReactNode, type RefObject } from "react";
import { DashboardOutline } from "../components/DashboardOutline";
import {
  chooseActiveDashboardOutlineItem,
  collectDashboardOutline,
  toDashboardOutlineItems,
  type DashboardOutlineItem,
} from "../components/dashboard-outline";

interface MockMutationObserverInstance {
  callback: MutationCallback;
  observe: ReturnType<typeof vi.fn>;
  disconnect: ReturnType<typeof vi.fn>;
}

interface MockIntersectionObserverInstance {
  callback: IntersectionObserverCallback;
  observe: ReturnType<typeof vi.fn>;
  disconnect: ReturnType<typeof vi.fn>;
}

const mutationObservers: MockMutationObserverInstance[] = [];
const intersectionObservers: MockIntersectionObserverInstance[] = [];

class MockMutationObserver implements MutationObserver {
  callback: MutationCallback;
  observe = vi.fn();
  disconnect = vi.fn();
  takeRecords = vi.fn(() => []);

  constructor(callback: MutationCallback) {
    this.callback = callback;
    mutationObservers.push(this);
  }
}

class MockIntersectionObserver implements IntersectionObserver {
  readonly root = null;
  readonly rootMargin = "";
  readonly scrollMargin = "";
  readonly thresholds = [];
  callback: IntersectionObserverCallback;
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  takeRecords = vi.fn(() => []);

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback;
    intersectionObservers.push(this);
  }
}

beforeEach(() => {
  mutationObservers.length = 0;
  intersectionObservers.length = 0;
  vi.stubGlobal("MutationObserver", MockMutationObserver);
  vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("DashboardOutline", () => {
  it("stays hidden until at least one outline item has been collected", () => {
    const rootRef = createDashboardRoot(<div />);

    render(<DashboardOutline rootRef={rootRef} />);

    expect(screen.queryByRole("navigation", { name: "Dashboard sections" })).not.toBeInTheDocument();
  });

  it("collects visible outline nodes from the dashboard root in DOM order", async () => {
    const rootRef = createDashboardRoot(
      <>
        <section id="summary" data-outline-title="Summary" />
        <section id="trends" data-outline-title="Trends" />
        <section id="breakdown" data-outline-title="Breakdown" />
      </>,
    );

    render(<DashboardOutline rootRef={rootRef} />);

    const links = await screen.findAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual(["Summary", "Trends", "Breakdown"]);
  });

  it("excludes matching nodes outside the dashboard root", async () => {
    appendVisibleElement(document.body, "outside", "Outside");
    const rootRef = createDashboardRoot(<section id="summary" data-outline-title="Summary" />);

    render(<DashboardOutline rootRef={rootRef} />);

    expect(await screen.findByRole("link", { name: "Summary" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Outside" })).not.toBeInTheDocument();
  });

  it("nests child items under containing parent items", async () => {
    const rootRef = createDashboardRoot(
      <section id="summary" data-outline-title="Summary">
        <section id="budget-allocations" data-outline-title="Budget Allocations" />
        <section id="spending-pace" data-outline-title="Spending Pace" />
        <section id="historical-comparison" data-outline-title="Historical Comparison" />
      </section>,
    );

    render(<DashboardOutline rootRef={rootRef} />);

    const summary = await screen.findByRole("link", { name: "Summary" });
    const childList = summary.closest("li")?.querySelector("ul");
    expect(childList).not.toBeNull();
    expect(childList).toHaveTextContent("Budget Allocations");
    expect(childList).toHaveTextContent("Spending Pace");
    expect(childList).toHaveTextContent("Historical Comparison");
  });

  it("keeps Trends and Breakdown as top-level links when they contain no outlineable children", async () => {
    const rootRef = createDashboardRoot(
      <>
        <section id="trends" data-outline-title="Trends">
          <div>Monthly Spending</div>
        </section>
        <section id="breakdown" data-outline-title="Breakdown">
          <div>Spending by Tag</div>
        </section>
      </>,
    );

    render(<DashboardOutline rootRef={rootRef} />);

    const trends = await screen.findByRole("link", { name: "Trends" });
    const breakdown = screen.getByRole("link", { name: "Breakdown" });
    expect(trends.closest("li")?.querySelector("ul")).toBeNull();
    expect(breakdown.closest("li")?.querySelector("ul")).toBeNull();
  });

  it("excludes hidden outline nodes", async () => {
    const rootRef = createDashboardRoot(
      <>
        <section id="summary" data-outline-title="Summary" />
        <section id="spending-pace" data-outline-title="Spending Pace" hidden />
      </>,
    );

    render(<DashboardOutline rootRef={rootRef} />);

    expect(await screen.findByRole("link", { name: "Summary" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Spending Pace" })).not.toBeInTheDocument();
  });

  it("adds conditional outline nodes after mutation observation", async () => {
    const rootRef = createDashboardRoot(<section id="summary" data-outline-title="Summary" />);

    render(<DashboardOutline rootRef={rootRef} />);
    expect(await screen.findByRole("link", { name: "Summary" })).toBeInTheDocument();

    appendVisibleElement(rootRef.current, "upcoming-prorata", "Upcoming Pro-rata");
    mutationObservers[0].callback([], mutationObservers[0] as unknown as MutationObserver);

    expect(await screen.findByRole("link", { name: "Upcoming Pro-rata" })).toBeInTheDocument();
  });

  it("updates outline nodes after observed attribute changes", async () => {
    const rootRef = createDashboardRoot(<section id="summary" data-outline-title="Summary" />);

    render(<DashboardOutline rootRef={rootRef} />);
    await screen.findByRole("link", { name: "Summary" });

    rootRef.current?.querySelector("#summary")?.setAttribute("data-outline-title", "Overview");
    mutationObservers[0].callback([], mutationObservers[0] as unknown as MutationObserver);

    expect(await screen.findByRole("link", { name: "Overview" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Summary" })).not.toBeInTheDocument();

    rootRef.current?.querySelector("#summary")?.setAttribute("hidden", "");
    mutationObservers[0].callback([], mutationObservers[0] as unknown as MutationObserver);

    await waitFor(() => {
      expect(screen.queryByRole("navigation", { name: "Dashboard sections" })).not.toBeInTheDocument();
    });
  });

  it("updates active link styling from intersection changes", async () => {
    const rootRef = createDashboardRoot(
      <>
        <section id="summary" data-outline-title="Summary" />
        <section id="trends" data-outline-title="Trends" />
      </>,
    );

    render(<DashboardOutline rootRef={rootRef} />);
    const summary = await screen.findByRole("link", { name: "Summary" });
    const trends = screen.getByRole("link", { name: "Trends" });

    emitIntersection("summary", true);
    await waitFor(() => expect(summary).toHaveAttribute("aria-current", "location"));

    emitIntersection("summary", false);
    emitIntersection("trends", true);
    await waitFor(() => expect(trends).toHaveAttribute("aria-current", "location"));
    expect(summary).not.toHaveAttribute("aria-current");
  });

  it("highlights the child when parent and child are both active", async () => {
    const rootRef = createDashboardRoot(
      <section id="summary" data-outline-title="Summary">
        <section id="spending-pace" data-outline-title="Spending Pace" />
      </section>,
    );

    render(<DashboardOutline rootRef={rootRef} />);
    const summary = await screen.findByRole("link", { name: "Summary" });
    const spendingPace = screen.getByRole("link", { name: "Spending Pace" });

    emitIntersection("summary", true);
    emitIntersection("spending-pace", true);

    await waitFor(() => expect(spendingPace).toHaveAttribute("aria-current", "location"));
    expect(summary).not.toHaveAttribute("aria-current");
  });
});

describe("collectDashboardOutline", () => {
  it("returns a visible DOM-derived containment tree through its public export", () => {
    const root = document.createElement("div");
    const summary = appendVisibleElement(root, "summary", "Summary");
    appendVisibleElement(summary, "budget-allocations", "Budget Allocations");
    appendVisibleElement(root, "trends", "Trends");
    appendVisibleElement(root, "hidden", "Hidden", { hidden: true });

    const items = toDashboardOutlineItems(collectDashboardOutline(root));

    expect(items).toEqual([
      {
        id: "summary",
        title: "Summary",
        children: [{ id: "budget-allocations", title: "Budget Allocations", children: [] }],
      },
      { id: "trends", title: "Trends", children: [] },
    ]);
  });

  it("excludes zero-size outline wrappers even when they have an offset parent", () => {
    const root = document.createElement("div");
    const emptySection = appendVisibleElement(root, "empty", "Empty");
    emptySection.getBoundingClientRect = vi.fn(() => ({
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      width: 0,
      height: 0,
      toJSON: () => ({}),
    }));
    Object.defineProperty(emptySection, "offsetParent", {
      configurable: true,
      value: document.body,
    });

    expect(toDashboardOutlineItems(collectDashboardOutline(root))).toEqual([]);
  });
});

describe("chooseActiveDashboardOutlineItem", () => {
  it("returns the deepest active item through its public export", () => {
    const items: DashboardOutlineItem[] = [
      {
        id: "summary",
        title: "Summary",
        children: [{ id: "spending-pace", title: "Spending Pace", children: [] }],
      },
    ];

    expect(chooseActiveDashboardOutlineItem(items, new Set(["summary", "spending-pace"]))).toBe(
      "spending-pace",
    );
  });
});

function createDashboardRoot(children: ReactNode): RefObject<HTMLDivElement | null> {
  const root = document.createElement("div");
  document.body.append(root);
  const rootRef = createRef<HTMLDivElement>();
  rootRef.current = root;
  render(<>{children}</>, { container: root });
  mockVisibleMeasurements(root);

  return rootRef;
}

function appendVisibleElement(
  parent: HTMLElement | null,
  id: string,
  title: string,
  options: { hidden?: boolean } = {},
): HTMLElement {
  if (!parent) throw new Error("Cannot append to missing parent");

  const element = document.createElement("section");
  element.id = id;
  element.dataset.outlineTitle = title;
  element.hidden = options.hidden ?? false;
  parent.append(element);
  mockVisibleMeasurements(element);

  return element;
}

function mockVisibleMeasurements(root: HTMLElement): void {
  const elements = [root, ...Array.from(root.querySelectorAll<HTMLElement>("*"))];

  for (const element of elements) {
    element.getBoundingClientRect = vi.fn(() => ({
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: element.hidden ? 0 : 100,
      bottom: element.hidden ? 0 : 40,
      width: element.hidden ? 0 : 100,
      height: element.hidden ? 0 : 40,
      toJSON: () => ({}),
    }));
  }
}

function buildIntersectionObserverEntry(
  target: Element,
  isIntersecting: boolean,
): IntersectionObserverEntry {
  const rect = target.getBoundingClientRect();
  return {
    target,
    isIntersecting,
    intersectionRatio: isIntersecting ? 1 : 0,
    boundingClientRect: rect,
    intersectionRect: rect,
    rootBounds: null,
    time: 0,
  };
}

function emitIntersection(id: string, isIntersecting: boolean): void {
  const element = document.getElementById(id);
  if (!element) throw new Error(`Missing element ${id}`);
  const observer = intersectionObservers.at(-1);
  if (!observer) throw new Error("Missing IntersectionObserver");

  observer.callback(
    [buildIntersectionObserverEntry(element, isIntersecting)],
    observer as unknown as IntersectionObserver,
  );
}
