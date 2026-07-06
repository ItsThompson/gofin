import { Button } from "@gofin/ui/components/button";
import { ArrowLeftToLine } from "lucide-react";
import type { ReturnToAdminButtonProps } from "./types";

/**
 * Floating control shown during identity assumption, letting the operator
 * restore their admin identity and return to /admin.
 */
export function ReturnToAdminButton({
  onReturn,
  disabled,
}: ReturnToAdminButtonProps) {
  return (
    <div className="fixed bottom-6 right-6 z-50">
      <Button
        variant="default"
        size="lg"
        onClick={onReturn}
        disabled={disabled}
        className="shadow-lg"
      >
        <ArrowLeftToLine className="size-4" />
        Return to Admin
      </Button>
    </div>
  );
}
