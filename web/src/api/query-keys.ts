import type { AdminStatsWindow } from "../types/admin";

export const queryKeys = {
  about: ["about"] as const,
  dashboard: ["dashboard"] as const,
  events: (limit: number) => ["events", limit] as const,
  providers: ["providers"] as const,
  providerDetail: (providerID: string | null) => ["provider-detail", providerID] as const,
  settings: ["settings"] as const,
  statsLogs: (params: {
    window: AdminStatsWindow;
    model: string;
    status: string;
    page: number;
    pageSize: number;
  }) => ["stats-logs", params.window, params.model, params.status, params.page, params.pageSize] as const,
  statsModels: (window: AdminStatsWindow) => ["stats-models", window] as const,
  statsProviders: (window: AdminStatsWindow) => ["stats-providers", window] as const,
  statsTimeline: (window: AdminStatsWindow, granularity: string) => ["stats-timeline", window, granularity] as const,
  statsSummary: (window: AdminStatsWindow) => ["stats-summary", window] as const,
};
