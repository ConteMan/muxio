import type { components } from "./schema";

export type Run = components["schemas"]["Run"];
export type RunEvent = components["schemas"]["RunEvent"];
export type RunPage = components["schemas"]["RunPage"];
export type RunEventPage = components["schemas"]["RunEventPage"];
export type Config = components["schemas"]["Config"];
export type ConfigSetting = components["schemas"]["ConfigSetting"];
export type ApiError = components["schemas"]["Error"];

/** RequestFailure carries the server's documented error shape. */
export class RequestFailure extends Error {
  readonly status: number;
  readonly code: string;
  readonly field?: string;

  constructor(status: number, code: string, message: string, field?: string) {
    super(message);
    this.name = "RequestFailure";
    this.status = status;
    this.code = code;
    this.field = field;
  }
}

async function request<T>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: "application/json" } });
  if (!response.ok) {
    // Every endpoint returns the same error shape, so one branch handles all.
    const body = (await response.json().catch(() => null)) as ApiError | null;
    throw new RequestFailure(
      response.status,
      body?.error ?? "internal",
      body?.message ?? `request failed with status ${response.status}`,
      body?.field,
    );
  }
  return (await response.json()) as T;
}

export function listRuns(params: { limit?: number; before?: number } = {}) {
  const query = new URLSearchParams();
  if (params.limit) query.set("limit", String(params.limit));
  if (params.before) query.set("before", String(params.before));
  const suffix = query.size > 0 ? `?${query}` : "";
  return request<RunPage>(`/api/v1/runs${suffix}`);
}

export function getRun(id: number) {
  return request<Run>(`/api/v1/runs/${id}`);
}

export function listRunEvents(id: number, params: { limit?: number; before?: number } = {}) {
  const query = new URLSearchParams();
  if (params.limit) query.set("limit", String(params.limit));
  if (params.before) query.set("before", String(params.before));
  const suffix = query.size > 0 ? `?${query}` : "";
  return request<RunEventPage>(`/api/v1/runs/${id}/events${suffix}`);
}

export function getConfig() {
  return request<Config>("/api/v1/config");
}
