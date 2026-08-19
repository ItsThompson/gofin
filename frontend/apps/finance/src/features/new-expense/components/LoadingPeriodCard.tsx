import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";

export function LoadingPeriodCard() {
  return (
    <div className="flex items-start justify-center pt-4 md:pt-8">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle className="text-2xl">New Expense</CardTitle>
          <CardDescription>Loading budget period context...</CardDescription>
        </CardHeader>
      </Card>
    </div>
  );
}
