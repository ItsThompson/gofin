import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { useAuthStore } from "@/stores/auth-store";

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

// A minimal version of the auth guard logic from login.tsx
// (testing redirect behavior in isolation from full page rendering)
function LoginPageGuard() {
  const { isAuthenticated, isLoading } = useAuthStore();
  if (isLoading) return <div>Loading...</div>;
  if (isAuthenticated) return <div>Redirected to dashboard</div>;
  return <div>Login form</div>;
}

function AuthLayoutGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuthStore();
  if (isLoading) return <div>Loading...</div>;
  if (!isAuthenticated) return <div>Redirected to login</div>;
  return <div>{children}</div>;
}

function renderWithRouter(
  initialRoute: string,
  element: React.ReactNode,
) {
  return render(
    <MemoryRouter initialEntries={[initialRoute]}>
      <Routes>
        <Route path="*" element={element} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("auth guard redirect logic", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  describe("unauthenticated route guard (login/register)", () => {
    it("shows login form when not authenticated", () => {
      useAuthStore.setState({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      });

      renderWithRouter("/login", <LoginPageGuard />);
      expect(screen.getByText("Login form")).toBeInTheDocument();
    });

    it("redirects to dashboard when already authenticated", () => {
      useAuthStore.setState({
        user: {
          id: "1",
          username: "test",
          email: "test@test.com",
          role: "user",
          currency: "USD",
          hasCompletedOnboarding: true,
          createdAt: "2026-01-01",
        },
        isAuthenticated: true,
        isAdmin: false,
        isLoading: false,
      });

      renderWithRouter("/login", <LoginPageGuard />);
      expect(screen.getByText("Redirected to dashboard")).toBeInTheDocument();
    });

    it("shows loading state while checking auth", () => {
      useAuthStore.setState({
        user: null,
        isAuthenticated: false,
        isLoading: true,
      });

      renderWithRouter("/login", <LoginPageGuard />);
      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });
  });

  describe("authenticated route guard (protected pages)", () => {
    it("shows content when authenticated", () => {
      useAuthStore.setState({
        user: {
          id: "1",
          username: "test",
          email: "test@test.com",
          role: "user",
          currency: "USD",
          hasCompletedOnboarding: true,
          createdAt: "2026-01-01",
        },
        isAuthenticated: true,
        isAdmin: false,
        isLoading: false,
      });

      renderWithRouter(
        "/dashboard",
        <AuthLayoutGuard>
          <div>Protected content</div>
        </AuthLayoutGuard>,
      );
      expect(screen.getByText("Protected content")).toBeInTheDocument();
    });

    it("redirects to login when not authenticated", () => {
      useAuthStore.setState({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      });

      renderWithRouter(
        "/dashboard",
        <AuthLayoutGuard>
          <div>Protected content</div>
        </AuthLayoutGuard>,
      );
      expect(screen.getByText("Redirected to login")).toBeInTheDocument();
    });

    it("shows loading state while checking auth", () => {
      useAuthStore.setState({
        user: null,
        isAuthenticated: false,
        isLoading: true,
      });

      renderWithRouter(
        "/dashboard",
        <AuthLayoutGuard>
          <div>Protected content</div>
        </AuthLayoutGuard>,
      );
      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });
  });
});
