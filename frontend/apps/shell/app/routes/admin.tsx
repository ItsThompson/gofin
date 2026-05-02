import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { useEffect, lazy, Suspense } from "react";

/**
 * Lazy-load the AdminPanelPage from the admin remote package.
 * This creates a code-split chunk: non-admin users never download this code
 * because the admin route guard redirects them before rendering.
 */
const AdminPanelPage = lazy(() =>
  import("@gofin/admin/src/pages/AdminPanelPage").then((mod) => ({
    default: mod.AdminPanelPage,
  })),
);

export default function AdminPage() {
  const { user, isAdmin, isLoading, assumeIdentity } = useAuthStore();
  const navigate = useNavigate();

  useEffect(() => {
    if (!isLoading && !isAdmin) {
      navigate("/dashboard");
    }
  }, [isLoading, isAdmin, navigate]);

  if (isLoading || !isAdmin) {
    return null;
  }

  const handleAssumeIdentity = async (userId: string) => {
    await assumeIdentity(userId);
    navigate("/dashboard");
  };

  return (
    <Suspense
      fallback={
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-muted-foreground">Loading admin panel...</div>
        </div>
      }
    >
      <AdminPanelPage
        currentUser={user}
        onAssumeIdentity={handleAssumeIdentity}
      />
    </Suspense>
  );
}
