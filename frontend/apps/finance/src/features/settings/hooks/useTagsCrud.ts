import { useState, useCallback, useEffect, type FormEvent } from "react";
import { ApiRequestError } from "@gofin/api";
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
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const fetchTags = useCallback(async () => {
    try {
      const response = await settingsApi.getTags();
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
        const response = await settingsApi.createTag(trimmed);
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
        const response = await settingsApi.updateTag(tagId, trimmed);
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
      await settingsApi.deleteTag(tagId);
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

  return {
    state: { tags, loading, newTagName, editingId, editingName, error, saving },
    actions: { setNewTagName, setEditingName, handleAddTag, handleStartEdit, handleCancelEdit, handleSaveEdit, handleDelete },
  };
}
