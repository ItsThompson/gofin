import { formatCurrency } from "@gofin/core";
import type { ProRataSchedule } from "@gofin/core";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import { Calendar } from "lucide-react";

interface UpcomingProRataSectionProps {
  schedules: ProRataSchedule[];
  currency: string;
}

export function UpcomingProRataSection({ schedules, currency }: UpcomingProRataSectionProps) {
  return (
    <Card data-testid="upcoming-prorata">
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <Calendar className="size-4 text-muted-foreground" />
          <CardTitle className="text-base">Upcoming Pro-rata</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className="divide-y">
          {schedules.map((schedule) => (
            <div
              key={schedule.id}
              className="flex items-center justify-between py-3 first:pt-0 last:pb-0"
            >
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-medium">{schedule.name}</span>
                <span className="text-xs text-muted-foreground">
                  Installment {schedule.installmentIndex} of {schedule.installmentTotal}
                </span>
              </div>
              <span className="text-sm font-semibold">
                {formatCurrency(schedule.amount, currency)}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
