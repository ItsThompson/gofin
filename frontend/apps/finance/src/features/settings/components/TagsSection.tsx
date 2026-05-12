import { Button } from "@gofin/ui/components/button";
import { Input } from "@gofin/ui/components/input";
import {
  Check,
  Loader2,
  Pencil,
  Trash2,
  Plus,
  X,
  Shield,
} from "lucide-react";
import { useTagsCrud } from "../hooks/useTagsCrud";

export function TagsSection() {
  const { state, actions } = useTagsCrud();

  if (state.loading) {
    return (
      <div className="flex items-center gap-2 py-8 text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        <span>Loading tags...</span>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {state.error && (
        <p className="text-sm text-red-600" role="alert">{state.error}</p>
      )}

      {/* Add Tag Form */}
      <form onSubmit={actions.handleAddTag} className="flex gap-2">
        <Input
          type="text"
          placeholder="New tag name"
          value={state.newTagName}
          onChange={(event) => actions.setNewTagName(event.target.value)}
          maxLength={50}
          aria-label="New tag name"
          className="flex-1"
        />
        <Button type="submit" disabled={state.saving || !state.newTagName.trim()} size="sm">
          <Plus className="size-4" />
          Add Tag
        </Button>
      </form>

      {/* Tags List */}
      <ul className="divide-y" role="list">
        {state.tags.map((tag) => (
          <li key={tag.id} className="flex items-center justify-between py-2 gap-2">
            {state.editing?.id === tag.id ? (
              <div className="flex flex-1 items-center gap-2">
                <Input
                  type="text"
                  value={state.editing.name}
                  onChange={(event) => actions.setEditingValue(event.target.value)}
                  maxLength={50}
                  aria-label="Edit tag name"
                  className="flex-1"
                  autoFocus
                />
                <Button
                  type="button"
                  size="sm"
                  onClick={actions.handleSaveEdit}
                  disabled={state.saving || !state.editing.name.trim()}
                >
                  <Check className="size-3" />
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={actions.handleCancelEdit}
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
                    onClick={() => actions.handleStartEdit(tag)}
                    aria-label={`Edit ${tag.name}`}
                  >
                    <Pencil className="size-3" />
                  </Button>
                  {!tag.isDefault && (
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => actions.handleDelete(tag.id)}
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

      {state.tags.length === 0 && (
        <p className="text-sm text-muted-foreground">No tags yet. Add one above.</p>
      )}
    </div>
  );
}
