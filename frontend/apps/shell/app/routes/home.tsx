import { Navigate } from "react-router";
import { getLandingPath } from "@gofin/core";
import { useAuthStore } from "@/stores/auth-store";

/** Root index route: send each identity to its role-aware landing path. */
export const handle = { access: "authenticated" as const };

export default function HomePage() {
  const { user } = useAuthStore();
  if (!user) return null;
  return <Navigate to={getLandingPath(user)} replace />;
}
