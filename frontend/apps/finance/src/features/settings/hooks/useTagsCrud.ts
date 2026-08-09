import { useState, useCallback, useEffect, type FormEvent } from "react";
import {
  ApiRequestError,
  classifyApiFailure,
  isNetworkError,
  NETWORK_FAILURE,
  reportError,
  useFormMutation,
} from "@gofin/api";
import type { Tag } from "@gofin/core";
import { settingsApi } from "../api";

export interface EditingTag {
  /** ID of the tag being edited. */
  id: string;
  /** Current value of the editing input. */
  name: string;
}

export interface TagsCrudState {
  tags: Tag[];
  loading: boolean;
  newTagName: string;
  /** Currently editing tag, or null if not editing. */
  editing: EditingTag | null;
  error: string | null;
  /**
   * Set when the tag list could not be loaded. Separate from `error`, which
   * belongs to the add/rename/delete mutations: an empty list after a failed
   * load means something different to the user than an empty list.
   */
  loadError: string | null;
  saving: boolean;
}

export interface TagsCrudActions {
  setNewTagName: (value: string) => void;
  /** Update the name of the tag currently being edited. */
  setEditingValue: (name: string) => void;
  handleAddTag: (event: FormEvent) => void;
  handleStartEdit: (tag: Tag) => void;
  handleCancelEdit: () => void;
  handleSaveEdit: () => void;
  handleDelete: (tagId: string) => void;
}

export function useTagsCrud(): { state: TagsCrudState; actions: TagsCrudActions } {
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [newTagName, setNewTagName] = useState("");
  const [editing, setEditing] = useState<EditingTag | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  const { submit, error: mutationError, submitting: saving } = useFormMutation<void>();

  const fetchTags = useCallback(async () => {
    try {
      const response = await settingsApi.getTags();
      setTags(response.tags);
      setLoadError(null);
    } catch (err) {
      reportError(err, {
        ...(isNetworkError(err) ? NETWORK_FAILURE : classifyApiFailure(err)),
        op: "tag.list",
        domain: "expenses",
      });
      setLoadError("Could not load your tags. Refresh to try again.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTags();
  }, [fetchTags]);

  const handleAddTag = useCallback(
    (event: FormEvent) => {
      event.preventDefault();
      const trimmed = newTagName.trim();
      if (!trimmed) return;

      submit(async () => {
        try {
          const response = await settingsApi.createTag(trimmed);
          setTags((prev) => [...prev, response.tag].sort((a, b) => a.name.localeCompare(b.name)));
          setNewTagName("");
        } catch (err) {
          if (err instanceof ApiRequestError && err.code === "DUPLICATE_TAG") {
            throw new ApiRequestError(err.status, {
              code: err.code,
              message: `A tag named "${trimmed}" already exists.`,
            });
          }
          throw err;
        }
      });
    },
    [newTagName, submit],
  );

  const handleStartEdit = useCallback((tag: Tag) => {
    setEditing({ id: tag.id, name: tag.name });
  }, []);

  const handleCancelEdit = useCallback(() => {
    setEditing(null);
  }, []);

  const setEditingValue = useCallback((name: string) => {
    setEditing((prev) => (prev ? { ...prev, name } : prev));
  }, []);

  const handleSaveEdit = useCallback(
    () => {
      if (!editing) return;
      const trimmed = editing.name.trim();
      if (!trimmed) return;

      const tagId = editing.id;
      submit(async () => {
        try {
          const response = await settingsApi.updateTag(tagId, trimmed);
          setTags((prev) =>
            prev.map((tag) => (tag.id === tagId ? response.tag : tag)).sort((a, b) => a.name.localeCompare(b.name)),
          );
          setEditing(null);
        } catch (err) {
          if (err instanceof ApiRequestError && err.code === "DUPLICATE_TAG") {
            throw new ApiRequestError(err.status, {
              code: err.code,
              message: `A tag named "${trimmed}" already exists.`,
            });
          }
          throw err;
        }
      });
    },
    [editing, submit],
  );

  const handleDelete = useCallback(
    (tagId: string) => {
      submit(async () => {
        try {
          await settingsApi.deleteTag(tagId);
          setTags((prev) => prev.filter((tag) => tag.id !== tagId));
        } catch (err) {
          if (err instanceof ApiRequestError) {
            if (err.code === "DEFAULT_TAG") {
              throw new ApiRequestError(err.status, {
                code: err.code,
                message: "Default tags cannot be deleted, only renamed.",
              });
            }
          }
          throw err;
        }
      });
    },
    [submit],
  );

  return {
    state: { tags, loading, newTagName, editing, error: mutationError, loadError, saving },
    actions: { setNewTagName, setEditingValue, handleAddTag, handleStartEdit, handleCancelEdit, handleSaveEdit, handleDelete },
  };
}
