import { startTransition } from "react";
import { hydrateRoot } from "react-dom/client";
import { HydratedRouter } from "react-router/dom";

async function boot() {
  if (import.meta.env.VITE_MOCK_API === "true") {
    const { worker } = await import("../mocks/browser");
    await worker.start({
      onUnhandledRequest: "bypass",
      quiet: false,
    });
    console.log(
      "%c[MSW] Mock API active: all /api/* requests are intercepted",
      "color: #10b981; font-weight: bold",
    );
  }

  startTransition(() => {
    hydrateRoot(document, <HydratedRouter />);
  });
}

boot();
