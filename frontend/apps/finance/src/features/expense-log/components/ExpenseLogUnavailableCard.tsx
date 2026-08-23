import { AlertTriangle, CalendarX2 } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";

interface ExpenseLogUnavailableCardProps {
  year: number;
  month: number;
  errorMessage?: string;
}

export function ExpenseLogUnavailableCard({
  year,
  month,
  errorMessage,
}: ExpenseLogUnavailableCardProps) {
  const monthName = new Date(year, month - 1).toLocaleString("en-US", {
    month: "long",
    year: "numeric",
  });

  return (
    <div className="flex items-start justify-center pt-8">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle className="text-2xl">
            {errorMessage ?? "No budget period for this month"}
          </CardTitle>
          {!errorMessage && (
            <CardDescription>
              There is no budget period for {monthName}.
            </CardDescription>
          )}
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-3 py-6 text-center">
          {errorMessage ? (
            <AlertTriangle
              className="size-8 text-muted-foreground/50"
              aria-hidden="true"
            />
          ) : (
            <CalendarX2
              className="size-8 text-muted-foreground/50"
              aria-hidden="true"
            />
          )}
          {!errorMessage && (
            <p className="text-sm text-muted-foreground">
              Select a different period to view expenses.
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
