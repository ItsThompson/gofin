import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RemoteLoadError } from "../src/components/RemoteLoadError";

describe("RemoteLoadError", () => {
  const reloadMock = vi.fn();

  beforeEach(() => {
    Object.defineProperty(window, "location", {
      value: { reload: reloadMock },
      writable: true,
    });
  });

  afterEach(() => {
    reloadMock.mockReset();
  });

  it("displays the section name in the error message", () => {
    render(<RemoteLoadError sectionName="Dashboard" />);

    expect(screen.getByText("Could not load Dashboard")).toBeInTheDocument();
  });

  it("displays a generic label when sectionName is not provided", () => {
    render(<RemoteLoadError />);

    expect(
      screen.getByText("Could not load this section"),
    ).toBeInTheDocument();
  });

  it("renders a helpful troubleshooting message", () => {
    render(<RemoteLoadError sectionName="Admin Panel" />);

    expect(
      screen.getByText(
        "Try refreshing the page. If the problem persists, check your connection.",
      ),
    ).toBeInTheDocument();
  });

  it("renders with alert role for accessibility", () => {
    render(<RemoteLoadError sectionName="Settings" />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("calls window.location.reload when the retry button is clicked", async () => {
    const user = userEvent.setup();
    render(<RemoteLoadError sectionName="Dashboard" />);

    const retryButton = screen.getByRole("button", { name: /refresh page/i });
    await user.click(retryButton);

    expect(reloadMock).toHaveBeenCalledTimes(1);
  });
});
