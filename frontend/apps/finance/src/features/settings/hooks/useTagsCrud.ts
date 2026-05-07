import { useState, useCallback, useEffect, type FormEvent } from "react";
import { ApiRequestError, useFormMutation } from "@gofin/api";
import type { Tag } from "@/types";
import { settingsApi } from "../api";

export interface TagsCrudState {
  tags: Tag[];
  loading: boolean;
  newTagName: string;
  editingId: string | null;
  editingName: string;
  error: string | null;
  saving: boolean;
}

export interface TagsCrudActions {
  setNewTagName: (value: string) => void;
  setEditingName: (value: string) => void;
  handleAddTag: (event: FormEvent) => void;
  handleStartEdit: (tag: Tag) => void;
  handleCancelEdit: () => void;
  handleSaveEdit: (tagId: string) => void;
  handleDelete: (tagId: string) => void;
}

export function useTagsCrud(): { state: TagsCrudState; actions: TagsCrudActions } {
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [newTagName, setNewTagName] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");

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
    setEditingId(tag.id);
    setEditingName(tag.name);
  }, []);

  const handleCancelEdit = useCallback(() => {
    setEditingId(null);
    setEditingName("");
  }, []);

  const handleSaveEdit = useCallback(
    (tagId: string) => {
      const trimmed = editingName.trim();
      if (!trimmed) return;

      submit(async () => {
        try {
          const response = await settingsApi.updateTag(tagId, trimmed);
          setTags((prev) =>
            prev.map((tag) => (tag.id === tagId ? response.tag : tag)).sort((a, b) => a.name.localeCompare(b.name)),
          );
          setEditingId(null);
          setEditingName("");
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
    [editingName, submit],
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
    state: { tags, loading, newTagName, editingId, editingName, error: mutationError, saving },
    actions: { setNewTagName, setEditingName, handleAddTag, handleStartEdit, handleCancelEdit, handleSaveEdit, handleDelete },
  };
}
