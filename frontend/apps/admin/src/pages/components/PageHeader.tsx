import { Shield } from "lucide-react";

export function PageHeader() {
  return (
    <div className="flex items-center gap-3">
      <Shield className="size-6 text-primary" />
      <div>
        <h1 className="text-2xl font-bold">Admin Panel</h1>
        <p className="text-sm text-muted-foreground">
          Manage users and assume identities for support
        </p>
      </div>
    </div>
  );
}
