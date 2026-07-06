import { NavLink } from "react-router";
import { Button } from "@gofin/ui/components/button";
import { PlusCircle } from "lucide-react";

/**
 * Mobile-only floating action button linking to the new-expense page. The
 * parent decides visibility (hidden for a direct admin, while assuming, and on
 * the new-expense page itself).
 */
export function LogExpenseFab() {
  return (
    <div className="fixed bottom-6 right-6 z-40 md:hidden">
      <NavLink to="/expenses/new">
        <Button
          size="lg"
          className="rounded-full shadow-lg size-14"
          aria-label="Log Expense"
        >
          <PlusCircle className="size-6" />
        </Button>
      </NavLink>
    </div>
  );
}
