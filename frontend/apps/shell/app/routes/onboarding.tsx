import { OnboardingFeature } from "@/features/onboarding";

export const handle = { access: "personal" as const };

export default function OnboardingRoute() {
  return <OnboardingFeature />;
}
