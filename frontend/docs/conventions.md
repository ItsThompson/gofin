# Frontend Conventions

## Feature Directory Structure

Every feature lives in its own directory under `apps/<app>/src/features/<name>/`:

```
features/<name>/
├── index.ts                 # Barrel: only public exports
├── <Name>Feature.tsx        # Orchestrator: composes hooks + layout
├── api.ts                   # Typed endpoint object
├── types.ts                 # Feature-local domain types (optional)
├── hooks/
│   └── use<Something>.ts    # One hook per file
├── components/
│   └── <Component>.tsx      # One component per file
└── __tests__/
    └── *.test.{ts,tsx}
```

### Rules

- **One exported function per TSX file.** Each component file exports exactly
  one React component. Helper components used only within that file may be
  defined locally but not exported.
- **Barrel exports define the public API.** Only re-export what other features
  or modules are allowed to import. Internal hooks, components, and utilities
  stay private.
- **Hooks encapsulate IO and state.** Components receive data via props and
  remain pure presentation layers. Hooks handle fetching, mutations, and state
  transitions.
- **Orchestrator wires hooks to children.** The top-level `<Name>Feature.tsx`
  calls hooks and passes results to child components. It contains no business
  logic of its own.

## The `api.ts` Pattern

Each feature defines a typed endpoint object that maps operation names to
`apiClient` calls:

```typescript
import { apiClient } from "@gofin/api";
import type { SummaryResponse, TagSpending } from "./types";

export const dashboardApi = {
  getSummary: (periodId: string) =>
    apiClient<SummaryResponse>(`/api/periods/${periodId}/summary`),

  getTagSpending: (periodId: string) =>
    apiClient<TagSpending[]>(`/api/periods/${periodId}/tags/spending`),
};
```

Benefits:
- Easy to mock at the boundary in tests (replace the object)
- No import coupling between features
- Each feature owns its endpoints

## Barrel Export Convention

`index.ts` at the feature root exports only what outside consumers need:

```typescript
// features/dashboard/index.ts
export { DashboardFeature } from "./DashboardFeature";
export { ActiveDashboard } from "./components/ActiveDashboard";
```

Everything not in the barrel is private to the feature. ESLint enforces this
(see "Boundary Enforcement" below).

## Boundary Enforcement (ESLint)

The shared ESLint config (`packages/config/eslint.config.js`) includes
`no-restricted-imports` rules that prevent reaching into feature internals:

| Restricted pattern | What it blocks |
|---|---|
| `**/features/*/components/*` | Direct component imports |
| `**/features/*/hooks/*` | Direct hook imports |
| `**/features/*/api` | Direct API module imports |

Importing from the feature barrel (`features/<name>`) is always allowed.

If you need to expose something from a feature to the outside, add it to
that feature's `index.ts` barrel.

## Adding a New Feature

Step-by-step example: adding an "analytics" feature to the finance app.

### 1. Create the directory structure

```
apps/finance/src/features/analytics/
├── index.ts
├── AnalyticsFeature.tsx
├── api.ts
├── hooks/
│   └── useAnalyticsData.ts
├── components/
│   └── SpendingBreakdown.tsx
└── __tests__/
    ├── useAnalyticsData.test.ts
    └── AnalyticsFeature.test.tsx
```

### 2. Define the API layer

```typescript
// api.ts
import { apiClient } from "@gofin/api";
import type { AnalyticsResponse } from "./types";

export const analyticsApi = {
  getBreakdown: (periodId: string) =>
    apiClient<AnalyticsResponse>(`/api/periods/${periodId}/analytics`),
};
```

### 3. Build the hook

```typescript
// hooks/useAnalyticsData.ts
import { useState, useEffect } from "react";
import { useApiToast } from "@gofin/api";
import { analyticsApi } from "../api";

export function useAnalyticsData(periodId: string) {
  // ... fetch and state management
}
```

### 4. Create the orchestrator

```typescript
// AnalyticsFeature.tsx
import { useAnalyticsData } from "./hooks/useAnalyticsData";
import { SpendingBreakdown } from "./components/SpendingBreakdown";

export function AnalyticsFeature({ periodId }: { periodId: string }) {
  const { data, loading, error } = useAnalyticsData(periodId);
  if (loading) return <Skeleton />;
  if (error) return <ErrorState />;
  return <SpendingBreakdown data={data} />;
}
```

### 5. Set up the barrel

```typescript
// index.ts
export { AnalyticsFeature } from "./AnalyticsFeature";
```

### 6. Add the route

```typescript
// routes.ts
const AnalyticsPage = lazy(() => import("./features/analytics"));
```

### 7. Write tests

Test hooks with `renderHook`, test the orchestrator with component rendering,
and mock only the `api.ts` object at the boundary.

## Coverage Thresholds

Coverage ratchets are configured per package in `vitest.config.ts`. Thresholds
are set at the measured floor: they only increase after real coverage gains, not
aspirationally. Current minimums:

| Package | Statements | Branches | Functions | Lines |
|---|---|---|---|---|
| `apps/finance` | 90% | 85% | 87% | 90% |
| `apps/shell` | 90% | 85% | 89% | 90% |
| `packages/core` | 95% | 90% | 95% | 95% |
| `packages/api` | 90% | 85% | 90% | 90% |
