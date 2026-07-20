import type { FaqItemContent } from "../types";

/**
 * A single FAQ entry: an <h3> question above a muted paragraph answer, sitting
 * beneath a top-border divider. Static Q/A only (no disclosure/expand-collapse
 * interaction).
 */
export function FaqItem({ question, answer }: FaqItemContent) {
  return (
    <div className="border-t py-6">
      <h3 className="font-heading text-lg font-semibold">{question}</h3>
      <p className="mt-2 text-muted-foreground">{answer}</p>
    </div>
  );
}
