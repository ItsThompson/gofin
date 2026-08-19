import { Button } from "@gofin/ui/components/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import { FormMessage } from "@gofin/ui/components/form";
import { LayoutDashboard } from "lucide-react";

interface MissingPeriodCardProps {
  year: number;
  month: number;
  errorMessage?: string;
}

export function MissingPeriodCard({
  year,
  month,
  errorMessage,
}: MissingPeriodCardProps) {
  return (
    <div className="flex items-start justify-center pt-4 md:pt-8">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <div className="flex items-center gap-3">
            <LayoutDashboard className="size-6 text-primary" />
            <CardTitle className="text-2xl">Create a budget period first</CardTitle>
          </div>
          <CardDescription>
            Expenses need a budget period for {month}/{year} before they can be saved.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {errorMessage ? <FormMessage>{errorMessage}</FormMessage> : null}
          <Button asChild className="w-full">
            <a href="/dashboard">Go to dashboard setup</a>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
