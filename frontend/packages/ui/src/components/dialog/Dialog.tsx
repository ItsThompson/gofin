import * as React from "react";
import { useEffect, useRef, useCallback } from "react";

interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
}

/**
 * Modal dialog overlay. Renders children over a backdrop that closes on click.
 * Traps focus and closes on Escape.
 */
export function Dialog({ open, onOpenChange, children }: DialogProps) {
  const overlayRef = useRef<HTMLDivElement>(null);

  const handleEscape = useCallback(
    (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onOpenChange(false);
      }
    },
    [onOpenChange],
  );

  useEffect(() => {
    if (open) {
      document.addEventListener("keydown", handleEscape);
      document.body.style.overflow = "hidden";
      return () => {
        document.removeEventListener("keydown", handleEscape);
        document.body.style.overflow = "";
      };
    }
  }, [open, handleEscape]);

  if (!open) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/60 p-4 pt-8 md:pt-16"
      onClick={(event) => {
        if (event.target === overlayRef.current) {
          onOpenChange(false);
        }
      }}
      role="dialog"
      aria-modal="true"
    >
      {children}
    </div>
  );
}
