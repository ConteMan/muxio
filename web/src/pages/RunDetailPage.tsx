import { useCallback } from "react";
import { ArrowLeft } from "lucide-react";
import { getRun, listRunEvents } from "../api/client";
import type { RunEvent } from "../api/client";
import { formatDuration, formatTime } from "../api/format";
import { Async, useAsync } from "../components/Async";
import { StatusBadge } from "../components/StatusBadge";

const levelStyles: Record<RunEvent["level"], string> = {
  info: "text-ink-muted",
  warn: "text-amber-700 dark:text-amber-300",
  error: "text-red-600 dark:text-red-400",
};

export function RunDetailPage({ id, onBack }: { id: number; onBack: () => void }) {
  const loadRun = useCallback(() => getRun(id), [id]);
  const loadEvents = useCallback(() => listRunEvents(id, { limit: 100 }), [id]);

  const [runState, reloadRun] = useAsync(loadRun, `run:${id}`);
  const [eventsState, reloadEvents] = useAsync(loadEvents, `events:${id}`);

  return (
    <section>
      <button
        type="button"
        onClick={onBack}
        className="mb-6 flex items-center gap-1.5 text-sm text-ink-muted hover:text-ink"
      >
        <ArrowLeft size={15} aria-hidden />
        Back to runs
      </button>

      <Async state={runState} onRetry={reloadRun}>
        {(run) => (
          <>
            <div className="mb-3 flex items-center gap-3">
              <h1 className="text-xl font-semibold tracking-tight">Run #{run.id}</h1>
              <StatusBadge status={run.status} />
            </div>

            <dl className="mb-8 grid grid-cols-2 gap-x-8 gap-y-3 rounded border border-line bg-surface p-5 text-sm sm:grid-cols-3">
              <Field label="Source" value={run.source_name} />
              <Field label="Trigger" value={run.trigger} />
              <Field
                label="Duration"
                value={formatDuration(run.started_at, run.finished_at) ?? "still running"}
              />
              <Field label="Started" value={formatTime(run.started_at)} title={run.started_at} />
              <Field
                label="Finished"
                value={formatTime(run.finished_at)}
                title={run.finished_at ?? undefined}
              />
              <Field label="Attempt" value={String(run.attempt)} />
              <Field label="Imported" value={String(run.imported_count)} />
              <Field label="Duplicate" value={String(run.duplicate_count)} />
              <Field label="Failed" value={String(run.failed_count)} />
            </dl>

            {run.last_error && (
              <p className="mb-8 rounded border border-red-500/30 bg-red-500/5 p-4 text-sm text-red-700 dark:text-red-300">
                {run.last_error}
              </p>
            )}
          </>
        )}
      </Async>

      <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-ink-muted">Events</h2>
      <Async state={eventsState} onRetry={reloadEvents}>
        {(page) =>
          page.items.length === 0 ? (
            <p className="rounded border border-line bg-surface p-5 text-sm text-ink-muted">
              No events recorded for this run.
            </p>
          ) : (
            <ol className="overflow-hidden rounded border border-line bg-surface">
              {page.items.map((event) => (
                <li
                  key={event.id}
                  className="flex gap-4 border-b border-line/60 px-4 py-2.5 text-sm last:border-0"
                >
                  <time
                    className="shrink-0 tabular-nums text-xs text-ink-muted"
                    dateTime={event.occurred_at}
                    title={event.occurred_at}
                  >
                    {formatTime(event.occurred_at)}
                  </time>
                  <span className={`w-12 shrink-0 text-xs font-medium ${levelStyles[event.level]}`}>
                    {event.level}
                  </span>
                  <span className="break-all">{event.message}</span>
                </li>
              ))}
            </ol>
          )
        }
      </Async>
    </section>
  );
}

function Field({ label, value, title }: { label: string; value: string; title?: string }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-ink-muted">{label}</dt>
      <dd className="mt-0.5 tabular-nums" title={title}>
        {value}
      </dd>
    </div>
  );
}
