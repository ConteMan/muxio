import { useEffect, useState } from "react";
import { Layout } from "./components/Layout";
import type { Tab } from "./components/Layout";
import { RunDetailPage } from "./pages/RunDetailPage";
import { RunsPage } from "./pages/RunsPage";
import { SettingsPage } from "./pages/SettingsPage";

type View = { name: "runs" } | { name: "run"; id: number } | { name: "settings" };

/**
 * Routing is read from the hash rather than the path: the server falls back to
 * index.html for unknown paths, and a hash keeps deep links working without any
 * routing library for three views.
 */
function readView(): View {
  const hash = window.location.hash.replace(/^#\/?/, "");
  if (hash === "settings") return { name: "settings" };
  const match = /^runs\/(\d+)$/.exec(hash);
  if (match) return { name: "run", id: Number(match[1]) };
  return { name: "runs" };
}

export function App() {
  const [view, setView] = useState<View>(readView);

  useEffect(() => {
    const onHashChange = () => setView(readView());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const navigate = (hash: string) => {
    window.location.hash = hash;
  };

  const tab: Tab = view.name === "settings" ? "settings" : "runs";

  return (
    <Layout tab={tab} onTabChange={(next) => navigate(next === "settings" ? "/settings" : "/runs")}>
      {view.name === "settings" && <SettingsPage />}
      {view.name === "runs" && <RunsPage onSelect={(id) => navigate(`/runs/${id}`)} />}
      {view.name === "run" && (
        <RunDetailPage id={view.id} onBack={() => navigate("/runs")} />
      )}
    </Layout>
  );
}
