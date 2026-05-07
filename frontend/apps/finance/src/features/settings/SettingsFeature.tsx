import { useState, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@gofin/ui/components/card";
import {
  Settings,
  Wallet,
  UserRound,
  Lock,
  Tags,
} from "lucide-react";
import type { SettingsPageProps } from "../../types";
import { DefaultBudgetSection } from "./components/DefaultBudgetSection";
import { ProfileSection } from "./components/ProfileSection";
import { PasswordSection } from "./components/PasswordSection";
import { TagsSection } from "./components/TagsSection";

type SettingsTab = "budget" | "profile" | "password" | "tags";

interface TabDefinition {
  id: SettingsTab;
  label: string;
  icon: typeof Wallet;
}

const TABS: TabDefinition[] = [
  { id: "budget", label: "Default Budget", icon: Wallet },
  { id: "profile", label: "Profile", icon: UserRound },
  { id: "password", label: "Password", icon: Lock },
  { id: "tags", label: "Tags", icon: Tags },
];

export function SettingsFeature({ user, onUserUpdated }: SettingsPageProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>("budget");
  const [expandedAccordion, setExpandedAccordion] = useState<SettingsTab | null>("budget");

  const toggleAccordion = useCallback((tab: SettingsTab) => {
    setExpandedAccordion((prev) => (prev === tab ? null : tab));
  }, []);

  const renderSection = useCallback(
    (tab: SettingsTab) => {
      switch (tab) {
        case "budget":
          return <DefaultBudgetSection user={user} />;
        case "profile":
          return <ProfileSection user={user} onUserUpdated={onUserUpdated} />;
        case "password":
          return <PasswordSection onUserUpdated={onUserUpdated} />;
        case "tags":
          return <TagsSection />;
      }
    },
    [user, onUserUpdated],
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Settings className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">Settings</h1>
      </div>

      {/* Desktop: tabbed layout */}
      <div className="hidden md:flex gap-6">
        <div className="flex flex-col gap-1 min-w-[180px]">
          {TABS.map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-left transition-colors ${
                  isActive
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                }`}
              >
                <Icon className="size-4" />
                {tab.label}
              </button>
            );
          })}
        </div>

        <Card className="flex-1">
          <CardHeader>
            <CardTitle>{TABS.find((tab) => tab.id === activeTab)?.label}</CardTitle>
          </CardHeader>
          <CardContent>{renderSection(activeTab)}</CardContent>
        </Card>
      </div>

      {/* Mobile: accordion sections */}
      <div className="flex flex-col gap-2 md:hidden">
        {TABS.map((tab) => {
          const Icon = tab.icon;
          const isExpanded = expandedAccordion === tab.id;
          return (
            <Card key={tab.id}>
              <button
                type="button"
                onClick={() => toggleAccordion(tab.id)}
                className="flex w-full items-center justify-between px-4 py-3 text-left"
              >
                <span className="flex items-center gap-2 text-sm font-medium">
                  <Icon className="size-4" />
                  {tab.label}
                </span>
                <span className="text-muted-foreground text-xs">
                  {isExpanded ? "▲" : "▼"}
                </span>
              </button>
              {isExpanded && (
                <CardContent className="border-t pt-4">
                  {renderSection(tab.id)}
                </CardContent>
              )}
            </Card>
          );
        })}
      </div>
    </div>
  );
}
