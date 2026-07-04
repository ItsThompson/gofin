import type { ReactNode } from "react";
import { Wallet, UserRound, Lock, Tags } from "lucide-react";
import type { User } from "@gofin/core";
import { canUseFinanceFeatures } from "@gofin/core";
import { DefaultBudgetSection } from "./components/DefaultBudgetSection";
import { ProfileSection } from "./components/ProfileSection";
import { PasswordSection } from "./components/PasswordSection";
import { TagsSection } from "./components/TagsSection";
import { ExportDataSection } from "./components/ExportDataSection";

export type SettingsTabId = "budget" | "profile" | "password" | "tags";

/** Props every section render receives; each render uses only what it needs. */
export interface SettingsSectionProps {
  user: User;
  onUserUpdated?: () => void;
}

export interface SettingsTabDefinition {
  id: SettingsTabId;
  label: string;
  icon: typeof Wallet;
  render: (props: SettingsSectionProps) => ReactNode;
}

// Shared, finance-agnostic tabs. The Profile tab here renders ProfileSection
// alone: it is the admin-safe variant (username/email only, no finance inputs).
const profileTab: SettingsTabDefinition = {
  id: "profile",
  label: "Profile",
  icon: UserRound,
  render: ({ user, onUserUpdated }) => (
    <ProfileSection user={user} onUserUpdated={onUserUpdated} />
  ),
};

const passwordTab: SettingsTabDefinition = {
  id: "password",
  label: "Password",
  icon: Lock,
  render: ({ onUserUpdated }) => <PasswordSection onUserUpdated={onUserUpdated} />,
};

// Admin (operator) tabs: Profile + Password only. The finance-only sections
// (DefaultBudgetSection, TagsSection, ExportDataSection) are never referenced
// here, so they are never rendered or called on the admin render path.
export const adminTabs: SettingsTabDefinition[] = [profileTab, passwordTab];

// Regular-user tabs: unchanged from the pre-refactor behavior. The finance-only
// sections are referenced exclusively by these definitions; the Profile tab
// additionally renders the data-export section.
export const userTabs: SettingsTabDefinition[] = [
  {
    id: "budget",
    label: "Default Budget",
    icon: Wallet,
    render: ({ user }) => <DefaultBudgetSection user={user} />,
  },
  {
    id: "profile",
    label: "Profile",
    icon: UserRound,
    render: ({ user, onUserUpdated }) => (
      <>
        <ProfileSection user={user} onUserUpdated={onUserUpdated} />
        <hr className="my-6 border-border" />
        <ExportDataSection />
      </>
    ),
  },
  passwordTab,
  {
    id: "tags",
    label: "Tags",
    icon: Tags,
    render: () => <TagsSection />,
  },
];

/** Role-derived tab list. Admins get the operator subset; users get the full set. */
export function getSettingsTabs(user: User): SettingsTabDefinition[] {
  return canUseFinanceFeatures(user) ? userTabs : adminTabs;
}
