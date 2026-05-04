import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/auth-store";
import { useEffect, lazy } from "react";
import { RemoteBoundary } from "@/components/remote-boundary";
import { Skeleton } from "@gofin/ui/components/skeleton";
import { Card, CardContent, CardHeader } from "@gofin/ui/components/card";

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

function AdminSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Skeleton className="size-6 rounded" />
        <div>
          <Skeleton className="h-8 w-32" />
          <Skeleton className="mt-1 h-4 w-56" />
        </div>
      </div>
      <Card>
        <CardHeader>
          <Skeleton className="h-6 w-40" />
          <Skeleton className="mt-1 h-4 w-64" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-10 w-48 rounded-md" />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <Skeleton className="h-6 w-36" />
          <Skeleton className="mt-1 h-4 w-28" />
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {Array.from({ length: 4 }).map((_, index) => (
              <div key={index} className="flex gap-4 border-b pb-3 last:border-0">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-36" />
                <Skeleton className="h-4 w-12" />
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-7 w-20 rounded-md" />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

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

  const grafanaUrl =
    typeof window !== "undefined" && window.location.hostname !== "localhost"
      ? `https://grafana.${window.location.hostname}`
      : "http://localhost:3001";

  return (
    <RemoteBoundary
      sectionName="Admin Panel"
      loadingFallback={<AdminSkeleton />}
    >
      <AdminPanelPage
        currentUser={user}
        onAssumeIdentity={handleAssumeIdentity}
        grafanaUrl={grafanaUrl}
      />
    </RemoteBoundary>
  );
}
