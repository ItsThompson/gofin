import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose,
} from "@gofin/ui/components/dialog";
import { BAND_LABEL } from "./healthScoreDisplay";

interface HealthScoreInfoModalProps {
  open: boolean;
  onClose: () => void;
}

const SUB_SCORES: { label: string; description: string }[] = [
  {
    label: "Savings",
    description: "How much of your savings target you hit this month.",
  },
  {
    label: "Budget adherence",
    description:
      "Whether your essentials and desires spending stayed within your plan.",
  },
  {
    label: "Allocation balance",
    description:
      "How closely your actual essentials/desires/savings split matched your target split.",
  },
  {
    label: "Spending stability",
    description:
      "How steady your discretionary (desires) spending is month to month. Needs at least three months of history.",
  },
];

const BANDS: { band: keyof typeof BAND_LABEL; range: string }[] = [
  { band: "green", range: "80-100" },
  { band: "amber", range: "55-79" },
  { band: "red", range: "0-54" },
];

/**
 * Learn-more modal explaining the health score, its sub-scores, the color bands,
 * and the provisional and building-baseline states. Static content sourced from
 * the ticket wording; reuses the shared Dialog (backdrop + focus trap + Escape).
 */
export function HealthScoreInfoModal({ open, onClose }: HealthScoreInfoModalProps) {
  return (
    <Dialog open={open} onOpenChange={() => onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>About your Financial Health Score</DialogTitle>
          <DialogClose onClick={onClose} />
        </DialogHeader>

        <div className="space-y-4 text-sm">
          <p className="text-muted-foreground">
            Your Financial Health Score is a single 0-100 read of the month, like
            a fitness tracker&apos;s readiness score. It sums a few sub-scores so
            you can see at a glance how the month went and what moved the number.
          </p>

          <div className="space-y-2">
            <h3 className="font-semibold">Sub-scores</h3>
            <ul className="space-y-2">
              {SUB_SCORES.map((subScore) => (
                <li key={subScore.label}>
                  <span className="font-medium">{subScore.label}:</span>{" "}
                  <span className="text-muted-foreground">
                    {subScore.description}
                  </span>
                </li>
              ))}
            </ul>
          </div>

          <div className="space-y-2">
            <h3 className="font-semibold">Bands</h3>
            <ul className="space-y-1">
              {BANDS.map(({ band, range }) => (
                <li key={band} className="text-muted-foreground">
                  <span className="font-medium text-foreground">
                    {BAND_LABEL[band]}
                  </span>{" "}
                  ({range})
                </li>
              ))}
            </ul>
          </div>

          <div className="space-y-2">
            <h3 className="font-semibold">Month to date &amp; building baseline</h3>
            <p className="text-muted-foreground">
              The current month is marked &quot;Month to date&quot; and firms up
              when the month closes. Spending stability needs three months of
              history; until then the card is &quot;building baseline&quot; and
              scores on the other sub-scores only.
            </p>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
