import { OnboardingFeature } from "@/features/onboarding";
import { accessHandle } from "@/lib/route-access";

export const handle = accessHandle("personal");

export default function OnboardingRoute() {
  return <OnboardingFeature />;
}
