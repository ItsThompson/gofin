import { Button } from "@gofin/ui/components/button";
import { Card, CardContent } from "@gofin/ui/components/card";
import { PlusCircle, Wallet } from "lucide-react";
import { Link } from "react-router";

interface EmptyExpensesProps {
  readOnly: boolean;
}

export function EmptyExpenses({ readOnly }: EmptyExpensesProps) {
  if (readOnly)
    return (
      <Card>
        <CardContent className="py-8 text-center text-sm text-muted-foreground">
          No expenses recorded for this period.
        </CardContent>
      </Card>
    );
  return (
    <Card>
      <CardContent className="flex flex-col items-center justify-center py-12 text-center">
        <Wallet className="mb-4 size-12 text-muted-foreground/50" />
        <h2 className="mb-2 text-lg font-semibold">No expenses yet</h2>
        <p className="mb-6 max-w-sm text-sm text-muted-foreground">
          Start tracking your spending by logging your first expense for this
          month.
        </p>
        <Button asChild>
          <Link to="/expenses/new">
            <PlusCircle className="size-4" />
            Log your first expense
          </Link>
        </Button>
      </CardContent>
    </Card>
  );
}
