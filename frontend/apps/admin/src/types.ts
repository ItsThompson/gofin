import type { User } from "@gofin/core";

export interface AdminPanelPageProps {
  currentUser: User | null;
  onAssumeIdentity: (userId: string) => Promise<void>;
  /** URL of the Grafana auth proxy. Defaults to http://localhost:3002. */
  grafanaUrl?: string;
}
