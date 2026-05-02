import { PlusCircle } from "lucide-react";

/**
 * Placeholder new expense page. Form implementation is wired in a later ticket.
 * Exported via Module Federation for the shell to load dynamically.
 */
export function NewExpensePage() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <PlusCircle className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">New Expense</h1>
      </div>
      <p className="text-muted-foreground">
        The expense form will appear here. Coming soon.
      </p>
    </div>
  );
}
