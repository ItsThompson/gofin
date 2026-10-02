import type { ProRataSchedule } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import { SectionState } from "../SectionState";
import { UpcomingProRataSection } from "../widgets/UpcomingProRataSection";
import type { DashboardSectionProps } from "./types";

export function UpcomingProRataDashboardSection({ enabled, refreshVersion, currency }: DashboardSectionProps) {
  const schedules = useDashboardRequest<readonly ProRataSchedule[]>(
    (signal, forceRefresh) => dashboardApi.getUpcomingProRata({ signal, forceRefresh }).then((response) => response.schedules),
    { enabled, refreshVersion, operation: "dashboard.upcomingProRata", emptyWhen: (data) => data.length === 0, forceRefreshOnVersionChange: true },
  );

  return (
    <section id="upcoming-prorata" data-outline-title="Upcoming Pro-rata">
      <SectionState label="Upcoming pro-rata" state={schedules.state} onRetry={schedules.retry} emptyMessage="No upcoming pro-rata payments.">
        {(data) => <SectionErrorBoundary sectionName="Upcoming Pro-rata"><UpcomingProRataSection schedules={[...data]} currency={currency} /></SectionErrorBoundary>}
      </SectionState>
    </section>
  );
}
