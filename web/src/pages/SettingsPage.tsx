import { useCallback } from "react";
import { getConfig } from "../api/client";
import type { ConfigSetting } from "../api/client";
import { Async, useAsync } from "../components/Async";

const originLabels: Record<ConfigSetting["origin"], string> = {
  default: "default",
  file: "config file",
  env: "environment",
  flag: "command line",
};

export function SettingsPage() {
  const load = useCallback(() => getConfig(), []);
  const [state, reload] = useAsync(load, "config");

  return (
    <section>
      <h1 className="mb-6 text-xl font-semibold tracking-tight">Settings</h1>
      <Async state={state} onRetry={reload}>
        {(config) => (
          <>
            <p className="mb-6 text-sm text-ink-muted">
              <code className="rounded bg-surface px-1.5 py-0.5">{config.path}</code>
              {!config.exists && " — not created yet, showing defaults"}
            </p>

            <div className="overflow-hidden rounded border border-line bg-surface">
              {config.settings.map((setting) => (
                <div
                  key={setting.key}
                  className="flex flex-wrap items-baseline gap-x-4 gap-y-1 border-b border-line/60 px-4 py-3 text-sm last:border-0"
                >
                  <span className="w-56 shrink-0 font-medium">{setting.key}</span>
                  <span className="tabular-nums">{setting.value}</span>
                  <span className="ml-auto text-xs text-ink-muted">
                    from {originLabels[setting.origin]}
                  </span>
                  {setting.origin === "env" && (
                    // A value overridden by the environment is not what the file
                    // says; without this the reader would edit the file in vain.
                    <span className="w-full text-xs text-amber-700 dark:text-amber-300">
                      Overridden by an environment variable — editing the file will not change
                      the effective value until that is unset.
                    </span>
                  )}
                </div>
              ))}
            </div>

            <p className="mt-6 text-sm text-ink-muted">
              Editing arrives in the next slice. For now use{" "}
              <code className="rounded bg-surface px-1.5 py-0.5">muxio config set</code>.
            </p>
          </>
        )}
      </Async>
    </section>
  );
}
