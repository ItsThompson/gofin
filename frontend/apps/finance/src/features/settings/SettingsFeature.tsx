import { useState, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@gofin/ui/components/card";
import { Settings } from "lucide-react";
import type { SettingsPageProps } from "../../types";
import { getSettingsTabs, type SettingsTabId } from "./settingsTabs";

export function SettingsFeature({ user, onUserUpdated }: SettingsPageProps) {
  const tabList = getSettingsTabs(user);
  const defaultTabId = tabList[0].id;

  const [activeTab, setActiveTab] = useState<SettingsTabId>(defaultTabId);
  const [expandedAccordion, setExpandedAccordion] = useState<SettingsTabId | null>(defaultTabId);

  const toggleAccordion = useCallback((tab: SettingsTabId) => {
    setExpandedAccordion((prev) => (prev === tab ? null : tab));
  }, []);

  const sectionProps = { user, onUserUpdated };
  const activeDefinition = tabList.find((tab) => tab.id === activeTab);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Settings className="size-6 text-primary" />
        <h1 className="text-2xl font-bold">Settings</h1>
      </div>

      {/* Desktop: tabbed layout */}
      <div className="hidden md:flex gap-6">
        <div className="flex flex-col gap-1 min-w-[180px]">
          {tabList.map((tab) => {
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
            <CardTitle>{activeDefinition?.label}</CardTitle>
          </CardHeader>
          <CardContent>{activeDefinition?.render(sectionProps)}</CardContent>
        </Card>
      </div>

      {/* Mobile: accordion sections */}
      <div className="flex flex-col gap-2 md:hidden">
        {tabList.map((tab) => {
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
                  {tab.render(sectionProps)}
                </CardContent>
              )}
            </Card>
          );
        })}
      </div>
    </div>
  );
}
