import { Navigate } from "react-router";

/** Root index route: redirect to /dashboard. */
export default function HomePage() {
  return <Navigate to="/dashboard" replace />;
}
