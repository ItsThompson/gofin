import { useNavigate } from "react-router";
import { getCurrencySymbol, formatCurrency } from "@gofin/core";
import type { TagSpending } from "@gofin/core";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

interface TagSpendingChartProps {
  tagSpending: TagSpending[];
  currency: string;
}

export function TagSpendingChart({ tagSpending, currency }: TagSpendingChartProps) {
  const navigate = useNavigate();

  const chartData = tagSpending.map((tag) => ({
    name: tag.tagName,
    amount: tag.amount / 100,
    tagId: tag.tagId,
    percent: tag.percentOfTotal,
  }));

  function handleBarClick(data: { tagId?: string }) {
    if (data.tagId) {
      navigate(`/expenses?tag=${data.tagId}`);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Spending by Tag</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={Math.max(200, tagSpending.length * 40)}>
          <BarChart
            data={chartData}
            layout="vertical"
            margin={{ top: 0, right: 20, left: 10, bottom: 0 }}
          >
            <CartesianGrid strokeDasharray="3 3" horizontal={false} />
            <XAxis type="number" tickFormatter={(value) => `${getCurrencySymbol(currency)}${value}`} />
            <YAxis type="category" dataKey="name" width={100} tick={{ fontSize: 12 }} />
            <Tooltip
              formatter={(value, _name, props) => [
                `${formatCurrency((value as number) * 100, currency)} (${(props as { payload: { percent: number } }).payload.percent.toFixed(1)}%)`,
                "Spent",
              ]}
            />
            <Bar
              dataKey="amount"
              fill="var(--primary)"
              radius={[0, 4, 4, 0]}
              cursor="pointer"
              onClick={(_data: unknown, index: number) => handleBarClick(chartData[index])}
            />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
