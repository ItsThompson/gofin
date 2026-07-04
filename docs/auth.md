# Auth System

## Overview

Authentication and authorization span two layers: the **Auth Service** (Go, Node 2) handles credentials, JWT lifecycle, and RBAC; the **Shell App** (Node 1) shares auth state with remote apps via a Zustand store. The **API Gateway** mediates every request by validating tokens via gRPC before routing downstream.

## JWT Token Architecture

| Token | Cookie Name | Path Scope | Purpose |
|-------|-------------|------------|---------|
| Access token | `gofin_access` | `/api` | Authenticates API requests (short-lived) |
| Refresh token | `gofin_refresh` | `/api/auth/refresh` | Obtains new access tokens (long-lived) |

Both cookies are `HttpOnly`, `Secure`, and `SameSite=Strict`. JavaScript cannot access them (XSS protection), and they are only sent over HTTPS in production. The refresh token's narrow path scope prevents it from being sent on non-refresh requests. Token lifetimes are configured in the Auth Service.

### Access Token Claims

```
sub         : User ID (UUID)
role        : "user" | "admin"
username    : Display name
iat         : Issued-at timestamp (Unix seconds)
exp         : Expiration timestamp (Unix seconds)
assumedBy?  : Admin's user ID (present during identity assumption)
```

### Refresh Token Claims

```
sub  : User ID (UUID)
jti  : Unique token ID (for blacklisting)
iat  : Issued-at timestamp
exp  : Expiration timestamp
```

## RBAC Model

| Role | Permissions |
|------|-------------|
| `user` | Full access to own financial data. No access to other users' data, the admin panel, or Grafana. |
| `admin` | Operator-only identity. Scope is authentication, the admin panel (user list), identity assumption, user deletion, and Grafana access. An admin owns no personal finance data and does not go through the onboarding or budget flows; the personal finance APIs return 403 to a direct admin. |

### Enforcement Points

| Layer | Mechanism |
|-------|-----------|
| API Gateway | Applies one centralized `AccessControl` middleware backed by an ordered policy table. Validates the JWT via Auth Service gRPC (401 on missing/invalid/expired), resolves each route to one of four access levels (Public / Authenticated / Personal / Admin), and enforces the role that level requires (403 on mismatch). Strips client-supplied identity headers, then passes `X-User-ID`, `X-User-Role`, and (when assuming) `X-Assumed-By` to downstream services. |
| Auth Service | Validates signature, expiration, and revocation status. Checks `tokens_revoked_at` on the user record: tokens with `iat` before this timestamp are rejected (handles password change invalidation). |
| Downstream Services | Trust the gateway. Scope all queries to the `user_id` from gateway headers. |
| Shell App | Client-side route guards as defense in depth (not sole enforcement). Hides admin routes for non-admin users. |
| Grafana Auth Proxy | Validates JWT locally using the shared signing secret. Checks `role === 'admin'`. Proxies to Grafana with `X-WEBAUTH-USER` header. |

### Gateway Access Levels

The gateway classifies every route into one of four access levels via a single ordered policy table (`services/gateway/internal/access`). A route is resolved by exact match first, then the longest matching prefix, else the fail-safe default of `Authenticated`:

| Level | Meaning | Token required | Role check |
|-------|---------|----------------|------------|
| `Public` | Reachable with no token | No | None |
| `Authenticated` | Any valid token | Yes | None |
| `Personal` | Valid token acting as a regular user | Yes | `role == "user"` |
| `Admin` | Valid token acting as an operator | Yes | `role == "admin"` |

The `Personal` routes are `/api/finance/*`, `/api/expenses/*`, `/api/datarights/exports*`, and `POST /api/auth/onboarding-complete`. A direct admin (`role=admin`) receives 403 on these routes; an assumed session carries `role=user` (plus an `assumedBy` claim), so it satisfies `Personal` and passes unchanged. `POST /api/auth/restore` is `Authenticated`, so an assumed session can always restore regardless of role.

This single `AccessControl` middleware replaced the former trio of mechanisms, all now removed: the `unauthenticatedRoutes` allowlist, `RequireAdmin`, and `AdminRouteGuard` (with its `adminOnlyRoutes`/`adminOnlyPrefixes`).

## Auth Flows

### Registration

1. User submits username, email, and password
2. Auth Service validates uniqueness and password strength (configurable; see Auth Service code for current rules)
3. Password is bcrypt-hashed and user record is created
4. JWT pair is generated, set as httpOnly cookies
5. User is redirected to onboarding

### Login

1. User submits email and password
2. Auth Service verifies the password hash
3. JWT pair is generated and set as cookies
4. User is redirected to the dashboard (or onboarding if not completed)

### Token Refresh

1. Frontend receives a 401 on an API call (access token expired)
2. Frontend automatically calls `POST /api/auth/refresh` (refresh cookie sent automatically)
3. Auth Service checks the blacklist, blacklists the old refresh token, and issues a new pair
4. Frontend retries the original request with the new access token
5. If the refresh token is also expired/blacklisted, the user is redirected to login

### Password Change

1. User provides current password and new password
2. Auth Service verifies the current password and validates the new one
3. A `tokens_revoked_at` timestamp is set on the user record
4. All tokens issued before this timestamp are now invalid (forces re-login on other sessions)
5. The current session receives freshly issued tokens

### Logout

1. Auth Service blacklists the refresh token
2. Both cookies are cleared
3. User is redirected to login

## Identity Assumption

Allows admins to view the app as another user for support and debugging.

### Assuming

1. Admin clicks "Assume" on a user in the admin panel
2. Shell stores the admin's user object in `originalAdminUser` (Zustand store, not localStorage)
3. `POST /api/auth/assume` is called with the target user ID
4. Auth Service generates a new JWT with the target user's `sub` and `role`, plus an `assumedBy` claim containing the admin's user ID
5. New cookies are set; the shell updates state (`user = target, isAssuming = true`)
6. UI navigates to the target user's dashboard

### Restoring

1. Admin clicks "Return to Admin" in the floating navbar banner
2. `POST /api/auth/restore` is called; the `assumedBy` claim identifies the admin
3. Auth Service generates fresh tokens for the admin identity
4. Shell restores `originalAdminUser` and clears `isAssuming`
5. UI navigates back to the admin panel

### Audit

All requests during assumption include the `assumedBy` claim. The API Gateway logs this, creating an audit trail: "admin X performed action Y as user Z."

## Grafana Auth Proxy

The auth proxy gates Grafana access to admin users only. It validates JWTs locally using the same signing secret as the Auth Service (no gRPC call needed), keeping the observability node independent of the compute node at runtime.

Flow: Browser → Cloudflare Tunnel → Auth Proxy (validate JWT, check admin role) → Grafana (with `X-WEBAUTH-USER` header).
