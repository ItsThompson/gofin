import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import { Activity, ExternalLink } from "lucide-react";

interface GrafanaCardProps {
  grafanaUrl: string;
}

export function GrafanaCard({ grafanaUrl }: GrafanaCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>System Monitoring</CardTitle>
        <CardDescription>
          View system health, service metrics, and performance dashboards
        </CardDescription>
      </CardHeader>
      <CardContent>
        <a
          href={grafanaUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
        >
          <Activity className="size-4" />
          Open Grafana Dashboards
          <ExternalLink className="size-3" />
        </a>
      </CardContent>
    </Card>
  );
}
