import { apiClient } from "@gofin/api";
import type {
  DefaultsResponse,
  UpdateDefaultsRequest,
  TagListResponse,
  TagResponse,
} from "@/types";

/** Request body for PUT /api/auth/me. */
export interface UpdateProfileRequest {
  username: string;
  email: string;
  currency: string;
}

/** Request body for POST /api/auth/me/password. */
export interface ChangePasswordRequest {
  currentPassword: string;
  newPassword: string;
}

/** Auth response wrapping a User (used by profile update, password change). */
export interface AuthResponse {
  user: import("@gofin/core").User;
}

export const settingsApi = {
  getDefaults: () =>
    apiClient<DefaultsResponse>("/api/finance/defaults"),

  updateDefaults: (body: UpdateDefaultsRequest) =>
    apiClient<DefaultsResponse>("/api/finance/defaults", {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  updateProfile: (body: UpdateProfileRequest) =>
    apiClient<AuthResponse>("/api/auth/me", {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  getProfile: () =>
    apiClient<AuthResponse>("/api/auth/me"),

  changePassword: (body: ChangePasswordRequest) =>
    apiClient<AuthResponse>("/api/auth/me/password", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  getTags: () =>
    apiClient<TagListResponse>("/api/finance/tags"),

  createTag: (name: string) =>
    apiClient<TagResponse>("/api/finance/tags", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),

  updateTag: (tagId: string, name: string) =>
    apiClient<TagResponse>(`/api/finance/tags/${tagId}`, {
      method: "PUT",
      body: JSON.stringify({ name }),
    }),

  deleteTag: (tagId: string) =>
    apiClient(`/api/finance/tags/${tagId}`, { method: "DELETE" }),
};
