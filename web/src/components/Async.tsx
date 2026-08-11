import type { ReactNode } from "react";
import { useCallback, useEffect, useState } from "react";
import { RequestFailure } from "../api/client";

export type AsyncState<T> =
  | { kind: "loading" }
  | { kind: "ready"; value: T }
  | { kind: "failed"; error: string };

/** Loads once per key change and exposes an explicit reload. */
export function useAsync<T>(load: () => Promise<T>, key: string): [AsyncState<T>, () => void] {
  const [state, setState] = useState<AsyncState<T>>({ kind: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let current = true;
    setState({ kind: "loading" });
    load()
      .then((value) => {
        if (current) setState({ kind: "ready", value });
      })
      .catch((error: unknown) => {
        if (!current) return;
        const message =
          error instanceof RequestFailure || error instanceof Error
            ? error.message
            : "the request failed";
        setState({ kind: "failed", error: message });
      });
    return () => {
      current = false;
    };
    // load is rebuilt on every render, so the key is what identifies the request.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, attempt]);

  const reload = useCallback(() => setAttempt((value) => value + 1), []);
  return [state, reload];
}

export function Async<T>({
  state,
  onRetry,
  children,
}: {
  state: AsyncState<T>;
  onRetry?: () => void;
  children: (value: T) => ReactNode;
}) {
  if (state.kind === "loading") {
    return <p className="py-8 text-sm text-ink-muted">Loading…</p>;
  }
  if (state.kind === "failed") {
    return (
      <div className="rounded border border-line bg-surface p-4">
        <p className="text-sm text-red-600 dark:text-red-400">{state.error}</p>
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="mt-3 rounded border border-line px-3 py-1 text-sm hover:bg-surface-sunken"
          >
            Retry
          </button>
        )}
      </div>
    );
  }
  return <>{children(state.value)}</>;
}
