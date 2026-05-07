import { Button } from "@gofin/ui/components/button";
import {
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";

export interface WelcomeStepProps {
  onNext: () => void;
}

export function WelcomeStep({ onNext }: WelcomeStepProps) {
  return (
    <>
      <CardHeader>
        <CardTitle className="text-2xl">Welcome to GoFin 🎉</CardTitle>
        <CardDescription>
          Let&apos;s set up your budget in a few quick steps. You can
          always change these later in Settings.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-3">
          <Button onClick={onNext} className="w-full">
            Get started
          </Button>
          <p className="text-center text-xs text-muted-foreground">
            Step 1 of 4
          </p>
        </div>
      </CardContent>
    </>
  );
}
