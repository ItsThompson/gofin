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

  switch (periodState.status) {
    case "loading":
      return <DashboardSkeleton />;

    case "error":
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

    case "no-period":
      return (
        <CreatePeriodPrompt
          defaults={periodState.defaults}
          user={user}
          year={currentYear}
          month={currentMonth}
          onCreatePeriod={periodState.createPeriod}
          creating={periodState.creating}
          createError={periodState.createError}
        />
      );

    case "active":
      return <ActiveDashboard period={periodState.period} user={user} />;
  }
}
