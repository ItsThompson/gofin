import { Navigate } from "react-router";

/**
 * /admin/users redirects to /admin which contains the full admin panel.
 * The admin panel page handles user list display.
 */
export default function AdminUsersPage() {
  return <Navigate to="/admin" replace />;
}
