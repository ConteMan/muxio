import type { Run } from "../api/client";

/**
 * Status is carried by text as well as colour: colour alone would be lost on a
 * monochrome display or to a reader who cannot distinguish it.
 */
const styles: Record<Run["status"], string> = {
  succeeded: "bg-emerald-500/12 text-emerald-700 dark:text-emerald-300",
  partial: "bg-amber-500/12 text-amber-700 dark:text-amber-300",
  failed: "bg-red-500/12 text-red-700 dark:text-red-300",
  interrupted: "bg-orange-500/12 text-orange-700 dark:text-orange-300",
  canceled: "bg-slate-500/12 text-ink-muted",
  running: "bg-blue-500/12 text-blue-700 dark:text-blue-300",
  queued: "bg-slate-500/12 text-ink-muted",
};

export function StatusBadge({ status }: { status: Run["status"] }) {
  return (
    <span
      className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${styles[status]}`}
    >
      {status}
    </span>
  );
}
