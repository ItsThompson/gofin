import { describe, it, expect } from "vitest";
import React from "react";
import { screen } from "@testing-library/react";
import { useParams, useSearchParams } from "react-router";
import { renderWithRouter } from "../render";

function LocationDisplay() {
  return React.createElement("div", { "data-testid": "location" }, "Rendered");
}

function ParamDisplay() {
  const params = useParams();
  return React.createElement("div", { "data-testid": "params" }, JSON.stringify(params));
}

function SearchParamDisplay() {
  const [searchParams] = useSearchParams();
  return React.createElement(
    "div",
    { "data-testid": "search" },
    searchParams.get("expired") ?? "none",
  );
}

describe("renderWithRouter", () => {
  it("renders a component within MemoryRouter", () => {
    renderWithRouter(React.createElement(LocationDisplay));

    const element = screen.getByTestId("location");
    expect(element.textContent).toBe("Rendered");
  });

  it("supports custom route path", () => {
    renderWithRouter(React.createElement(LocationDisplay), {
      route: "/login",
    });

    const element = screen.getByTestId("location");
    expect(element.textContent).toBe("Rendered");
  });

  it("supports searchParams", () => {
    renderWithRouter(React.createElement(SearchParamDisplay), {
      route: "/login",
      searchParams: { expired: "true" },
    });

    const element = screen.getByTestId("search");
    expect(element.textContent).toBe("true");
  });

  it("supports routeConfig for useParams matching", () => {
    renderWithRouter(React.createElement("div"), {
      route: "/expenses/exp-123",
      routeConfig: [
        {
          path: "/expenses/:id",
          element: React.createElement(ParamDisplay),
        },
      ],
    });

    const element = screen.getByTestId("params");
    expect(element.textContent).toContain('"id":"exp-123"');
  });
});
