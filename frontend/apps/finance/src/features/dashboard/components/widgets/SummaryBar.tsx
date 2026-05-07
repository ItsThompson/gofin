import { formatCurrency } from "@gofin/core";
import {
  Card,
  CardContent,
} from "@gofin/ui/components/card";
import {
  Wallet,
  TrendingDown,
  Calendar,
} from "lucide-react";
import { getRemainingColor } from "../../../../lib/budget-utils";

interface SummaryBarProps {
  budgetAmount: number;
  totalSpent: number;
  remaining: number;
  daysLeft: number;
  currency: string;
}

export function SummaryBar({
  budgetAmount,
  totalSpent,
  remaining,
  daysLeft,
  currency,
}: SummaryBarProps) {
  const remainingColor = getRemainingColor(budgetAmount, remaining);

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      <SummaryCard
        label="Total Budget"
        value={formatCurrency(budgetAmount, currency)}
        icon={<Wallet className="size-4 text-muted-foreground" />}
      />
      <SummaryCard
        label="Total Spent"
        value={formatCurrency(totalSpent, currency)}
        icon={<TrendingDown className="size-4 text-muted-foreground" />}
      />
      <SummaryCard
        label="Remaining"
        value={formatCurrency(remaining, currency)}
        icon={<Wallet className="size-4 text-muted-foreground" />}
        valueClassName={remainingColor}
      />
      <SummaryCard
        label="Days Left"
        value={String(daysLeft)}
        icon={<Calendar className="size-4 text-muted-foreground" />}
      />
    </div>
  );
}

function SummaryCard({
  label,
  value,
  icon,
  valueClassName,
}: {
  label: string;
  value: string;
  icon: React.ReactNode;
  valueClassName?: string;
}) {
  return (
    <Card>
      <CardContent className="px-4 py-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {icon}
          {label}
        </div>
        <p className={`mt-1 text-xl font-bold ${valueClassName ?? ""}`}>
          {value}
        </p>
      </CardContent>
    </Card>
  );
}
