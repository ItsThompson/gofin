import type { User } from "@gofin/core";
import { DashboardSkeleton } from "@gofin/ui/components/skeletons";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@gofin/ui/components/card";
import { Button } from "@gofin/ui/components/button";
import { usePeriodState } from "./hooks/usePeriodState";
import { CreatePeriodPrompt } from "./components/CreatePeriodPrompt";
import { ActiveDashboard } from "./components/ActiveDashboard";

interface DashboardFeatureProps {
  user: User;
}

export function DashboardFeature({ user }: DashboardFeatureProps) {
  const periodState = usePeriodState();

  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  if (periodState.state === "loading") {
    return <DashboardSkeleton />;
  }

  if (periodState.state === "error") {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-destructive">Something went wrong</CardTitle>
          <CardDescription>
            Could not load the dashboard. The error details were shown in a
            notification.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" onClick={periodState.retry}>
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (periodState.state === "no-period") {
    return (
      <CreatePeriodPrompt
        defaults={periodState.defaults}
        user={user}
        year={currentYear}
        month={currentMonth}
        onPeriodCreated={periodState.handlePeriodCreated}
      />
    );
  }

  return <ActiveDashboard period={periodState.period!} user={user} />;
}
