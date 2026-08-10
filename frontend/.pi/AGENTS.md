# GoFin Frontend

## Directory Conventions

- `apps/shell/` → `app/` directory (React Router file-based routing, SSR)
- `apps/finance/`, `apps/admin/` → `src/` directory (standard Vite; the shell imports their source as workspace packages)

## Route Registration

All 3 levels are required for a page to be user-accessible:

1. **Finance package**: `apps/finance/src/routes.ts` (declares the route)
2. **Shell route config**: `apps/shell/app/routes.ts` + matching `routes/<name>.tsx` file (registers in React Router)
3. **Nav links**: `apps/shell/app/routes/auth-layout.tsx` navLinks array (makes it discoverable)

## Feature Extraction Checklist

1. Create feature in `apps/finance/src/features/<name>/`
2. Export in the feature's `index.ts`
3. Add to `apps/finance/src/routes.ts`
4. Create `apps/shell/app/routes/<name>.tsx` (lazy-load pattern)
5. Register in `apps/shell/app/routes.ts` inside the layout array
6. (Optional) Add to navLinks in auth-layout.tsx

## Frontend Composition

- The shell imports feature source directly: `@gofin/finance/src/features/<name>/index`
- Composition is build-time, so a new feature needs no registration beyond the route files above
- Never add a runtime remote loader: one bundle from one origin, so a release is a single artifact rather than three that must be kept in step

## Path Aliases

- `@/` imports in feature source files **break** when the shell builds them (shell's Vite alias takes precedence over finance/admin's)
- Files in `features/X/` importing from `features/Y/` must use relative paths (`../Y`)
- Tailwind v4: add `@source` directives in each app's CSS for workspace package source directories

## Testing

```bash
# Per-package (correct):
cd apps/finance && npx vitest run
cd apps/shell && npx vitest run

# Or via turbo:
npx turbo test

# No workspace-root vitest config exists: never run vitest from frontend/
```

### DOM Testing Patterns
- `fireEvent.submit(form)` bypasses HTML5 `required` validation (use when testing app's own validation logic)
- `fireEvent.change(select, { target: { value } })` for native `<select>` (not `user.selectOptions`)
- Dual-render responsive pages (desktop table + mobile list): use `getAllByText` not `getByText`
- URL-based `createMockApi` for tests with parallel fetches (no sequential mock counting)

### Shell App Type Checking
```bash
npx react-router typegen && npx tsc -b   # must run before committing shell changes
```
