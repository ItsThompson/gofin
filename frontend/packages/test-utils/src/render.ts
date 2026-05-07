import React from "react";
import { render } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import type { RenderOptions, RenderResult } from "@testing-library/react";

export interface RouterRenderOptions extends RenderOptions {
  /** Initial route path. Defaults to "/". */
  route?: string;
  /** Search params to append to the route. */
  searchParams?: Record<string, string>;
  /** Route configuration for useParams matching. */
  routeConfig?: Array<{ path: string; element: React.ReactElement }>;
}

/**
 * Render a component within MemoryRouter context.
 * Returns the standard @testing-library/react render result.
 *
 * When routeConfig is provided, the component is rendered within a Routes/Route
 * structure to enable useParams matching. Otherwise, the component is rendered
 * as a direct child of MemoryRouter.
 */
export function renderWithRouter(
  ui: React.ReactElement,
  options?: RouterRenderOptions,
): RenderResult {
  const {
    route = "/",
    searchParams,
    routeConfig,
    ...renderOptions
  } = options ?? {};

  let initialEntry = route;
  if (searchParams) {
    const params = new URLSearchParams(searchParams);
    initialEntry = `${route}?${params.toString()}`;
  }

  if (routeConfig) {
    return render(
      React.createElement(
        MemoryRouter,
        { initialEntries: [initialEntry] },
        React.createElement(
          Routes,
          null,
          ...routeConfig.map((config) =>
            React.createElement(Route, {
              key: config.path,
              path: config.path,
              element: config.element,
            }),
          ),
        ),
      ),
      renderOptions,
    );
  }

  return render(
    React.createElement(MemoryRouter, { initialEntries: [initialEntry] }, ui),
    renderOptions,
  );
}
