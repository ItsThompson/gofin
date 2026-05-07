import { useState, useCallback } from "react";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import { Label } from "@gofin/ui/components/label";
import { useMultiStepDialog } from "@gofin/ui/components/multi-step-dialog";
import { apiClient, ApiRequestError } from "@gofin/api";
import { Loader2 } from "lucide-react";
import type { PasswordStepProps } from "../types";

export function PasswordStep({ userId, username, onSuccess, onOpenChange }: PasswordStepProps) {
  const { back } = useMultiStepDialog();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    setError("");
    setIsSubmitting(true);

    try {
      await apiClient(`/api/admin/users/${userId}`, {
        method: "DELETE",
        body: JSON.stringify({ password }),
      });
      onOpenChange(false);
      onSuccess();
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred");
      }
    } finally {
      setIsSubmitting(false);
    }
  }, [userId, password, onSuccess, onOpenChange]);

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
        {error && (
          <p className="text-sm text-destructive">{error}</p>
        )}
      </div>
      <div className="flex justify-between">
        <Button variant="outline" onClick={back} disabled={isSubmitting}>
          Back
        </Button>
        <Button
          variant="destructive"
          onClick={handleSubmit}
          disabled={!password || isSubmitting}
        >
          {isSubmitting && <Loader2 className="size-3 animate-spin" />}
          Delete User
        </Button>
      </div>
    </div>
  );
}
