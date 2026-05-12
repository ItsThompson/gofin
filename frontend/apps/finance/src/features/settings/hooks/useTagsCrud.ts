import { useState, useCallback, useEffect, type FormEvent } from "react";
import { ApiRequestError, useFormMutation } from "@gofin/api";
import type { Tag } from "../../../types";
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

  const { submit, error: mutationError, submitting: saving } = useFormMutation<void>();

  const fetchTags = useCallback(async () => {
    try {
      const response = await settingsApi.getTags();
      setTags(response.tags);
    } catch {
      // Tag fetch failure is non-critical; show inline error
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
    state: { tags, loading, newTagName, editing, error: mutationError, saving },
    actions: { setNewTagName, setEditingValue, handleAddTag, handleStartEdit, handleCancelEdit, handleSaveEdit, handleDelete },
  };
}
