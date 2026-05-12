import { useState, useCallback } from "react";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import { Label } from "@gofin/ui/components/label";
import { useMultiStepDialog } from "@gofin/ui/components/multi-step-dialog";
import { apiClient, useFormMutation } from "@gofin/api";
import { Loader2 } from "lucide-react";
import type { PasswordStepProps, DeletionJobResponse } from "../types";

export function PasswordStep({ userId, username, onSuccess, onOpenChange }: PasswordStepProps) {
  const { back } = useMultiStepDialog();
  const [password, setPassword] = useState("");

  const mutation = useFormMutation<DeletionJobResponse>({
    onSuccess: (job) => {
      onOpenChange(false);
      onSuccess(job);
    },
  });

  const handleSubmit = useCallback(() => {
    mutation.submit(() =>
      apiClient<DeletionJobResponse>("/api/datarights/deletions", {
        method: "POST",
        body: JSON.stringify({ userId, password }),
      }),
    );
  }, [userId, password, mutation]);

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Enter your admin password to confirm deletion of user{" "}
        <span className="font-semibold">{username}</span>.
      </p>
      <div className="space-y-2">
        <Label htmlFor="admin-password">Your Password</Label>
        <Input
          id="admin-password"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder="Enter your password"
          autoComplete="current-password"
        />
        {mutation.error && (
          <p className="text-sm text-destructive">{mutation.error}</p>
        )}
      </div>
      <div className="flex justify-between">
        <Button variant="outline" onClick={back} disabled={mutation.submitting}>
          Back
        </Button>
        <Button
          variant="destructive"
          onClick={handleSubmit}
          disabled={!password || mutation.submitting}
        >
          {mutation.submitting && <Loader2 className="size-3 animate-spin" />}
          Delete User
        </Button>
      </div>
    </div>
  );
}
