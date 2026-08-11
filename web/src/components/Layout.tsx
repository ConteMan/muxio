import type { ReactNode } from "react";
import { History, Settings } from "lucide-react";

export type Tab = "runs" | "settings";

export function Layout({
  tab,
  onTabChange,
  children,
}: {
  tab: Tab;
  onTabChange: (tab: Tab) => void;
  children: ReactNode;
}) {
  const tabs: Array<{ id: Tab; label: string; icon: typeof History }> = [
    { id: "runs", label: "Runs", icon: History },
    { id: "settings", label: "Settings", icon: Settings },
  ];

  return (
    <div className="min-h-screen">
      <header className="border-b border-line bg-surface">
        <div className="mx-auto flex max-w-5xl items-center gap-6 px-6 py-3">
          <span className="font-semibold tracking-tight">Muxio</span>
          <nav className="flex gap-1">
            {tabs.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                type="button"
                onClick={() => onTabChange(id)}
                aria-current={tab === id ? "page" : undefined}
                className={`flex items-center gap-1.5 rounded px-3 py-1.5 text-sm transition ${
                  tab === id
                    ? "bg-surface-sunken font-medium text-ink"
                    : "text-ink-muted hover:text-ink"
                }`}
              >
                <Icon size={15} aria-hidden />
                {label}
              </button>
            ))}
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
    </div>
  );
}
