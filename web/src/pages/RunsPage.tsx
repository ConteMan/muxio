import { useCallback, useState } from "react";
import { listRuns } from "../api/client";
import type { Run } from "../api/client";
import { formatDuration, formatTime } from "../api/format";
import { Async, useAsync } from "../components/Async";
import { StatusBadge } from "../components/StatusBadge";

const PAGE_SIZE = 20;

export function RunsPage({ onSelect }: { onSelect: (id: number) => void }) {
  // Cursors are kept as a stack so "previous" is exact rather than recomputed.
  const [cursors, setCursors] = useState<number[]>([]);
  const before = cursors.at(-1);

  const load = useCallback(() => listRuns({ limit: PAGE_SIZE, before }), [before]);
  const [state, reload] = useAsync(load, `runs:${before ?? "first"}`);

  return (
    <section>
      <h1 className="mb-6 text-xl font-semibold tracking-tight">Runs</h1>
      <Async state={state} onRetry={reload}>
        {(page) => (
          <>
            {page.items.length === 0 ? (
              <p className="rounded border border-line bg-surface p-6 text-sm text-ink-muted">
                No runs recorded yet. Import something with{" "}
                <code className="rounded bg-surface-sunken px-1">muxio import</code>.
              </p>
            ) : (
              <div className="overflow-x-auto rounded border border-line bg-surface">
                <table className="w-full text-sm">
                  <thead className="border-b border-line text-left text-xs uppercase tracking-wide text-ink-muted">
                    <tr>
                      <th className="px-4 py-2.5 font-medium">Run</th>
                      <th className="px-4 py-2.5 font-medium">Source</th>
                      <th className="px-4 py-2.5 font-medium">Status</th>
                      <th className="px-4 py-2.5 font-medium">Started</th>
                      <th className="px-4 py-2.5 text-right font-medium">Imported</th>
                      <th className="px-4 py-2.5 text-right font-medium">Duplicate</th>
                      <th className="px-4 py-2.5 text-right font-medium">Failed</th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.items.map((run: Run) => (
                      <tr
                        key={run.id}
                        onClick={() => onSelect(run.id)}
                        className="cursor-pointer border-b border-line/60 last:border-0 hover:bg-surface-sunken"
                      >
                        <td className="px-4 py-2.5 tabular-nums">#{run.id}</td>
                        <td className="px-4 py-2.5">{run.source_name}</td>
                        <td className="px-4 py-2.5">
                          <StatusBadge status={run.status} />
                        </td>
                        <td className="px-4 py-2.5 text-ink-muted" title={run.started_at}>
                          {formatTime(run.started_at)}
                          {formatDuration(run.started_at, run.finished_at) && (
                            <span className="ml-2 text-xs">
                              ({formatDuration(run.started_at, run.finished_at)})
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums">
                          {run.imported_count}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums text-ink-muted">
                          {run.duplicate_count}
                        </td>
                        <td
                          className={`px-4 py-2.5 text-right tabular-nums ${
                            run.failed_count > 0 ? "text-red-600 dark:text-red-400" : "text-ink-muted"
                          }`}
                        >
                          {run.failed_count}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            <div className="mt-4 flex gap-2">
              <button
                type="button"
                disabled={cursors.length === 0}
                onClick={() => setCursors((stack) => stack.slice(0, -1))}
                className="rounded border border-line px-3 py-1.5 text-sm disabled:opacity-40 enabled:hover:bg-surface"
              >
                Previous
              </button>
              <button
                type="button"
                disabled={page.next_before === null}
                onClick={() =>
                  page.next_before !== null &&
                  setCursors((stack) => [...stack, page.next_before as number])
                }
                className="rounded border border-line px-3 py-1.5 text-sm disabled:opacity-40 enabled:hover:bg-surface"
              >
                Next
              </button>
            </div>
          </>
        )}
      </Async>
    </section>
  );
}
