import { Navigate } from "react-router";

/**
 * /admin/users redirects to /admin which contains the full admin panel.
 * The admin panel page handles user list display.
 */
export const handle = { access: "admin" as const };

export default function AdminUsersPage() {
  return <Navigate to="/admin" replace />;
}
