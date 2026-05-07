/** A user-owned expense tag. */
export interface Tag {
  id: string;
  name: string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

/** Response from GET /api/finance/tags. */
export interface TagListResponse {
  tags: Tag[];
}

/** Response from POST/PUT /api/finance/tags. */
export interface TagResponse {
  tag: Tag;
}
