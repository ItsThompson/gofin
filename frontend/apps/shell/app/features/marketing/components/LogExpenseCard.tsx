import { motion } from "framer-motion";

/** The expense fields that autofill in scene 1, revealed in sequence. */
const EXPENSE_FIELDS = [
  { label: "Name", value: "Groceries" },
  { label: "Amount", value: "$84.20" },
  { label: "Category", value: "Essentials" },
  { label: "Tag", value: "Food" },
] as const;

/**
 * Scene 1 of the hero: a stylized "log expense" form whose fields autofill in
 * sequence before the "Log Expense" button depresses. Marketing-only, no real
 * form state or data. Rendered only on the animated path (the reduced-motion
 * path shows the dashboard end state instead), so it always animates.
 */
export function LogExpenseCard() {
  return (
    <div className="flex h-full w-full flex-col gap-4 rounded-xl bg-card p-6 text-card-foreground ring-1 ring-foreground/10">
      <span className="text-sm font-medium text-muted-foreground">
        Log expense
      </span>

      <div className="flex flex-col gap-3">
        {EXPENSE_FIELDS.map((field, index) => (
          <motion.div
            key={field.label}
            className="flex items-center justify-between rounded-lg bg-muted px-3 py-2"
            initial={{ opacity: 0, x: -8 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{
              duration: 0.4,
              delay: 0.2 + index * 0.35,
              repeat: Infinity,
              repeatType: "reverse",
              repeatDelay: 2.4,
            }}
          >
            <span className="text-xs font-medium text-muted-foreground">
              {field.label}
            </span>
            <span className="text-sm font-medium">{field.value}</span>
          </motion.div>
        ))}
      </div>

      <motion.div
        className="mt-auto rounded-lg bg-primary px-4 py-2 text-center text-sm font-medium text-primary-foreground"
        animate={{ scale: [1, 1, 0.96, 1] }}
        transition={{
          duration: 0.6,
          delay: 1.8,
          repeat: Infinity,
          repeatDelay: 2.2,
        }}
      >
        Log Expense
      </motion.div>
    </div>
  );
}
