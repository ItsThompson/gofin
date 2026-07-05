import { Navigate } from "react-router";
import { accessHandle } from "@/lib/route-access";

/**
 * /admin/users redirects to /admin which contains the full admin panel.
 * The admin panel page handles user list display.
 */
export const handle = accessHandle("admin");

export default function AdminUsersPage() {
  return <Navigate to="/admin" replace />;
}
