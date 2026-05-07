import { useState } from "react";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import { Label } from "@gofin/ui/components/label";
import { useMultiStepDialog } from "@gofin/ui/components/multi-step-dialog";

const CONFIRMATION_PHRASE = "permanently delete";

export function ConfirmationStep() {
  const { next } = useMultiStepDialog();
  const [confirmText, setConfirmText] = useState("");

  const isValid = confirmText === CONFIRMATION_PHRASE;

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        This action is permanent and cannot be undone. To confirm, type{" "}
        <span className="font-mono font-semibold text-destructive">{CONFIRMATION_PHRASE}</span>{" "}
        below.
      </p>
      <div className="space-y-2">
        <Label htmlFor="confirm-phrase">Confirmation</Label>
        <Input
          id="confirm-phrase"
          value={confirmText}
          onChange={(event) => setConfirmText(event.target.value)}
          placeholder={CONFIRMATION_PHRASE}
          autoComplete="off"
        />
      </div>
      <div className="flex justify-end">
        <Button onClick={next} disabled={!isValid}>
          Next
        </Button>
      </div>
    </div>
  );
}
