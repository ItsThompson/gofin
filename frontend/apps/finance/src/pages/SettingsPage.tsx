import { useState, useCallback, useEffect, type FormEvent } from "react";
import {
  apiClient,
  ApiRequestError,
  type User,
  type DefaultsResponse,
  type UpdateDefaultsRequest,
  type UpdateProfileRequest,
  type ChangePasswordRequest,
  type AuthResponse,
  type Tag,
  type TagListResponse,
  type TagResponse,
} from "@gofin/types";
import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import { Card, CardContent, CardHeader, CardTitle } from "@gofin/ui/components/card";
import {
  Form,
  FormField,
  FormLabel,
  FormMessage,
} from "@gofin/ui/components/form";
import {
  Settings,
  Wallet,
  UserRound,
  Lock,
  Tags,
  Check,
  Loader2,
  Pencil,
  Trash2,
  Plus,
  X,
  Shield,
} from "lucide-react";
import type { SettingsPageProps } from "../types";

const CURRENCY_OPTIONS = [
  { code: "USD", label: "USD ($)" },
  { code: "EUR", label: "EUR (€)" },
  { code: "GBP", label: "GBP (£)" },
  { code: "JPY", label: "JPY (¥)" },
  { code: "CAD", label: "CAD (C$)" },
  { code: "AUD", label: "AUD (A$)" },
  { code: "CHF", label: "CHF" },
  { code: "CNY", label: "CNY (¥)" },
  { code: "SGD", label: "SGD (S$)" },
  { code: "HKD", label: "HKD (HK$)" },
];

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

/**
 * Validates that E/D/S percentages sum to exactly 100.
 * Returns an error message or null if valid.
 */
function validateEDSSplit(essentials: number, desires: number, savings: number): string | null {
  const total = essentials + desires + savings;
  if (total !== 100) {
    return `Percentages must sum to 100% (currently ${total}%)`;
  }
  return null;
}

/**
 * Validates password strength: 8+ chars, 1 upper, 1 lower, 1 digit.
 */
function validatePasswordStrength(password: string): string | null {
  if (password.length < 8) {
    return "Password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit";
  }
  if (!/[A-Z]/.test(password)) {
    return "Password must contain at least one uppercase letter";
  }
  if (!/[a-z]/.test(password)) {
    return "Password must contain at least one lowercase letter";
  }
  if (!/\d/.test(password)) {
    return "Password must contain at least one digit";
  }
  return null;
}

// --- Section Components ---

function DefaultBudgetSection({ user }: { user: User }) {
  const [budgetDollars, setBudgetDollars] = useState("");
  const [essentials, setEssentials] = useState("");
  const [desires, setDesires] = useState("");
  const [savings, setSavings] = useState("");
  const [currency, setCurrency] = useState(user.currency);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [loading, setLoading] = useState(false);
  const [fetching, setFetching] = useState(true);

  useEffect(() => {
    async function fetchDefaults() {
      try {
        const response = await apiClient<DefaultsResponse>("/api/finance/defaults");
        const defaults = response.defaults;
        setBudgetDollars(String(defaults.budgetAmount / 100));
        setEssentials(String(defaults.essentialsPercent));
        setDesires(String(defaults.desiresPercent));
        setSavings(String(defaults.savingsPercent));
        setCurrency(defaults.currency);
      } catch {
        // Use fallback defaults
        setBudgetDollars("0");
        setEssentials("50");
        setDesires("30");
        setSavings("20");
      } finally {
        setFetching(false);
      }
    }
    fetchDefaults();
  }, []);

  const handleSubmit = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      setError(null);
      setSuccess(false);

      const essentialsNum = parseInt(essentials, 10) || 0;
      const desiresNum = parseInt(desires, 10) || 0;
      const savingsNum = parseInt(savings, 10) || 0;

      const splitError = validateEDSSplit(essentialsNum, desiresNum, savingsNum);
      if (splitError) {
        setError(splitError);
        return;
      }

      const budgetCents = Math.round((parseFloat(budgetDollars) || 0) * 100);

      setLoading(true);

      try {
        // Update finance defaults
        const body: UpdateDefaultsRequest = {
          budgetAmount: budgetCents,
          essentialsPercent: essentialsNum,
          desiresPercent: desiresNum,
          savingsPercent: savingsNum,
          currency,
        };

        await apiClient<DefaultsResponse>("/api/finance/defaults", {
          method: "PUT",
          body: JSON.stringify(body),
        });

        // Sync currency to auth service. Fetch the current profile
        // from the server to avoid sending stale username/email if the
        // user edited their profile in another tab before saving here.
        const currentProfile = await apiClient<AuthResponse>("/api/auth/me");
        const profileBody: UpdateProfileRequest = {
          username: currentProfile.user.username,
          email: currentProfile.user.email,
          currency,
        };

        await apiClient<AuthResponse>("/api/auth/me", {
          method: "PUT",
          body: JSON.stringify(profileBody),
        });

        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        if (err instanceof ApiRequestError) {
          setError(err.message);
        } else {
          setError("An unexpected error occurred. Please try again.");
        }
      } finally {
        setLoading(false);
      }
    },
    [budgetDollars, essentials, desires, savings, currency],
  );

  if (fetching) {
    return (
      <div className="flex items-center gap-2 py-8 text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        <span>Loading defaults...</span>
      </div>
    );
  }

  return (
    <Form onSubmit={handleSubmit}>
      <FormField>
        <FormLabel htmlFor="budget-amount">Monthly Budget</FormLabel>
        <Input
          id="budget-amount"
          type="number"
          min="0"
          step="0.01"
          value={budgetDollars}
          onChange={(event) => setBudgetDollars(event.target.value)}
        />
      </FormField>

      <div className="grid grid-cols-3 gap-3">
        <FormField>
          <FormLabel htmlFor="essentials-pct">Essentials %</FormLabel>
          <Input
            id="essentials-pct"
            type="number"
            min="0"
            max="100"
            value={essentials}
            onChange={(event) => setEssentials(event.target.value)}
          />
        </FormField>
        <FormField>
          <FormLabel htmlFor="desires-pct">Desires %</FormLabel>
          <Input
            id="desires-pct"
            type="number"
            min="0"
            max="100"
            value={desires}
            onChange={(event) => setDesires(event.target.value)}
          />
        </FormField>
        <FormField>
          <FormLabel htmlFor="savings-pct">Savings %</FormLabel>
          <Input
            id="savings-pct"
            type="number"
            min="0"
            max="100"
            value={savings}
            onChange={(event) => setSavings(event.target.value)}
          />
        </FormField>
      </div>

      <FormField>
        <FormLabel htmlFor="currency-select">Currency</FormLabel>
        <select
          id="currency-select"
          value={currency}
          onChange={(event) => setCurrency(event.target.value)}
          className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {CURRENCY_OPTIONS.map((opt) => (
            <option key={opt.code} value={opt.code}>
              {opt.label}
            </option>
          ))}
        </select>
      </FormField>

      <FormMessage>{error}</FormMessage>

      {success && (
        <p className="flex items-center gap-1.5 text-sm text-green-600">
          <Check className="size-4" />
          Default settings updated successfully.
        </p>
      )}

      <Button type="submit" disabled={loading}>
        {loading && <Loader2 className="size-4 animate-spin" />}
        Save Defaults
      </Button>
    </Form>
  );
}

function ProfileSection({ user, onUserUpdated }: { user: User; onUserUpdated?: () => void }) {
  const [username, setUsername] = useState(user.username);
  const [email, setEmail] = useState(user.email);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      setError(null);
      setSuccess(false);
      setLoading(true);

      try {
        const body: UpdateProfileRequest = {
          username: username.trim(),
          email: email.trim(),
          currency: user.currency,
        };

        await apiClient<AuthResponse>("/api/auth/me", {
          method: "PUT",
          body: JSON.stringify(body),
        });

        onUserUpdated?.();
        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        if (err instanceof ApiRequestError) {
          if (err.code === "DUPLICATE_EMAIL") {
            setError("An account with this email already exists.");
          } else if (err.code === "DUPLICATE_USERNAME") {
            setError("This username is already taken.");
          } else {
            setError(err.message);
          }
        } else {
          setError("An unexpected error occurred. Please try again.");
        }
      } finally {
        setLoading(false);
      }
    },
    [username, email, user.currency],
  );

  return (
    <Form onSubmit={handleSubmit}>
      <FormField>
        <FormLabel htmlFor="profile-username">Username</FormLabel>
        <Input
          id="profile-username"
          type="text"
          value={username}
          onChange={(event) => setUsername(event.target.value)}
          required
        />
      </FormField>

      <FormField>
        <FormLabel htmlFor="profile-email">Email</FormLabel>
        <Input
          id="profile-email"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />
      </FormField>

      <FormMessage>{error}</FormMessage>

      {success && (
        <p className="flex items-center gap-1.5 text-sm text-green-600">
          <Check className="size-4" />
          Profile updated successfully.
        </p>
      )}

      <Button type="submit" disabled={loading}>
        {loading && <Loader2 className="size-4 animate-spin" />}
        Update Profile
      </Button>
    </Form>
  );
}

function PasswordSection({ onUserUpdated }: { onUserUpdated?: () => void }) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      setError(null);
      setSuccess(false);

      // Client-side validation
      const strengthError = validatePasswordStrength(newPassword);
      if (strengthError) {
        setError(strengthError);
        return;
      }

      setLoading(true);

      try {
        const body: ChangePasswordRequest = {
          currentPassword,
          newPassword,
        };

        await apiClient<AuthResponse>("/api/auth/me/password", {
          method: "POST",
          body: JSON.stringify(body),
        });

        setCurrentPassword("");
        setNewPassword("");
        onUserUpdated?.();
        setSuccess(true);
        setTimeout(() => setSuccess(false), 3000);
      } catch (err) {
        if (err instanceof ApiRequestError) {
          if (err.code === "INVALID_CREDENTIALS") {
            setError("Current password is incorrect.");
          } else if (err.code === "WEAK_PASSWORD") {
            setError(err.message);
          } else {
            setError(err.message);
          }
        } else {
          setError("An unexpected error occurred. Please try again.");
        }
      } finally {
        setLoading(false);
      }
    },
    [currentPassword, newPassword],
  );

  return (
    <Form onSubmit={handleSubmit}>
      <FormField>
        <FormLabel htmlFor="current-password">Current Password</FormLabel>
        <Input
          id="current-password"
          type="password"
          value={currentPassword}
          onChange={(event) => setCurrentPassword(event.target.value)}
          required
        />
      </FormField>

      <FormField>
        <FormLabel htmlFor="new-password">New Password</FormLabel>
        <Input
          id="new-password"
          type="password"
          value={newPassword}
          onChange={(event) => setNewPassword(event.target.value)}
          required
        />
        <p className="text-xs text-muted-foreground">
          Minimum 8 characters with at least one uppercase letter, one lowercase letter, and one digit.
        </p>
      </FormField>

      <FormMessage>{error}</FormMessage>

      {success && (
        <p className="flex items-center gap-1.5 text-sm text-green-600">
          <Check className="size-4" />
          Password changed successfully. Other sessions have been signed out.
        </p>
      )}

      <Button type="submit" disabled={loading}>
        {loading && <Loader2 className="size-4 animate-spin" />}
        Change Password
      </Button>
    </Form>
  );
}

function TagsSection() {
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [newTagName, setNewTagName] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const fetchTags = useCallback(async () => {
    try {
      const response = await apiClient<TagListResponse>("/api/finance/tags");
      setTags(response.tags);
    } catch {
      setError("Failed to load tags.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTags();
  }, [fetchTags]);

  const handleAddTag = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      const trimmed = newTagName.trim();
      if (!trimmed) return;
      setError(null);
      setSaving(true);

      try {
        const response = await apiClient<TagResponse>("/api/finance/tags", {
          method: "POST",
          body: JSON.stringify({ name: trimmed }),
        });
        setTags((prev) => [...prev, response.tag].sort((a, b) => a.name.localeCompare(b.name)));
        setNewTagName("");
      } catch (err) {
        if (err instanceof ApiRequestError) {
          setError(err.code === "DUPLICATE_TAG" ? `A tag named "${trimmed}" already exists.` : err.message);
        } else {
          setError("Failed to create tag.");
        }
      } finally {
        setSaving(false);
      }
    },
    [newTagName],
  );

  const handleStartEdit = useCallback((tag: Tag) => {
    setEditingId(tag.id);
    setEditingName(tag.name);
    setError(null);
  }, []);

  const handleCancelEdit = useCallback(() => {
    setEditingId(null);
    setEditingName("");
  }, []);

  const handleSaveEdit = useCallback(
    async (tagId: string) => {
      const trimmed = editingName.trim();
      if (!trimmed) return;
      setError(null);
      setSaving(true);

      try {
        const response = await apiClient<TagResponse>(`/api/finance/tags/${tagId}`, {
          method: "PUT",
          body: JSON.stringify({ name: trimmed }),
        });
        setTags((prev) =>
          prev.map((tag) => (tag.id === tagId ? response.tag : tag)).sort((a, b) => a.name.localeCompare(b.name)),
        );
        setEditingId(null);
        setEditingName("");
      } catch (err) {
        if (err instanceof ApiRequestError) {
          setError(err.code === "DUPLICATE_TAG" ? `A tag named "${trimmed}" already exists.` : err.message);
        } else {
          setError("Failed to rename tag.");
        }
      } finally {
        setSaving(false);
      }
    },
    [editingName],
  );

  const handleDelete = useCallback(async (tagId: string) => {
    setError(null);
    try {
      await apiClient(`/api/finance/tags/${tagId}`, { method: "DELETE" });
      setTags((prev) => prev.filter((tag) => tag.id !== tagId));
    } catch (err) {
      if (err instanceof ApiRequestError) {
        if (err.code === "DEFAULT_TAG") {
          setError("Default tags cannot be deleted, only renamed.");
        } else if (err.code === "TAG_IN_USE") {
          setError(err.message);
        } else {
          setError(err.message);
        }
      } else {
        setError("Failed to delete tag.");
      }
    }
  }, []);

  if (loading) {
    return (
      <div className="flex items-center gap-2 py-8 text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        <span>Loading tags...</span>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <p className="text-sm text-red-600" role="alert">{error}</p>
      )}

      {/* Add Tag Form */}
      <form onSubmit={handleAddTag} className="flex gap-2">
        <Input
          type="text"
          placeholder="New tag name"
          value={newTagName}
          onChange={(event) => setNewTagName(event.target.value)}
          maxLength={50}
          aria-label="New tag name"
          className="flex-1"
        />
        <Button type="submit" disabled={saving || !newTagName.trim()} size="sm">
          <Plus className="size-4" />
          Add Tag
        </Button>
      </form>

      {/* Tags List */}
      <ul className="divide-y" role="list">
        {tags.map((tag) => (
          <li key={tag.id} className="flex items-center justify-between py-2 gap-2">
            {editingId === tag.id ? (
              <div className="flex flex-1 items-center gap-2">
                <Input
                  type="text"
                  value={editingName}
                  onChange={(event) => setEditingName(event.target.value)}
                  maxLength={50}
                  aria-label="Edit tag name"
                  className="flex-1"
                  autoFocus
                />
                <Button
                  type="button"
                  size="sm"
                  onClick={() => handleSaveEdit(tag.id)}
                  disabled={saving || !editingName.trim()}
                >
                  <Check className="size-3" />
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={handleCancelEdit}
                >
                  <X className="size-3" />
                </Button>
              </div>
            ) : (
              <>
                <div className="flex items-center gap-2">
                  <span className="text-sm">{tag.name}</span>
                  {tag.isDefault && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
                      <Shield className="size-3" />
                      Default
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => handleStartEdit(tag)}
                    aria-label={`Edit ${tag.name}`}
                  >
                    <Pencil className="size-3" />
                  </Button>
                  {!tag.isDefault && (
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => handleDelete(tag.id)}
                      aria-label={`Delete ${tag.name}`}
                      className="text-red-600 hover:text-red-700"
                    >
                      <Trash2 className="size-3" />
                    </Button>
                  )}
                </div>
              </>
            )}
          </li>
        ))}
      </ul>

      {tags.length === 0 && (
        <p className="text-sm text-muted-foreground">No tags yet. Add one above.</p>
      )}
    </div>
  );
}

// --- Main Settings Page ---

/**
 * Settings page with four sections: Default Budget, Profile, Password, Tags.
 *
 * Desktop: tabbed layout. Mobile: accordion sections.
 * Exported via Module Federation for the shell to load dynamically.
 */
export function SettingsPage({ user, onUserUpdated }: SettingsPageProps) {
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
