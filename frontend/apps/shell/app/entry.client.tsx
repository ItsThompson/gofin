import { isNetworkError } from "@gofin/api";
import { loadSupportedCurrencies } from "@gofin/core";
import * as Sentry from "@sentry/react-router";
import { startTransition } from "react";
import { hydrateRoot } from "react-dom/client";
import { HydratedRouter } from "react-router/dom";
import { useAuthStore } from "@/stores/auth-store";
import { clientOptions } from "../sentry.options.mjs";

async function boot() {
  // DSN presence is the only switch. NODE_ENV cannot serve: the E2E stack runs
  // as production by design and one of its specs routes an endpoint to a 500,
  // so the CI image is built without the build arg instead.
  const dsn = import.meta.env.VITE_SENTRY_DSN;
  if (dsn) {
    Sentry.init(
      clientOptions({
        dsn,
        release: import.meta.env.VITE_SENTRY_RELEASE,
        isNetworkError,
      }),
    );
  }

  // The currency catalog endpoint requires a session and login happens inside
  // the SPA without a page reload, so load it on auth transitions instead of
  // firing once at module load (which would 401 for logged-out visitors).
  useAuthStore.subscribe((state, previousState) => {
    if (!previousState.isAuthenticated && state.isAuthenticated) {
      void loadSupportedCurrencies();
    }
  });

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
