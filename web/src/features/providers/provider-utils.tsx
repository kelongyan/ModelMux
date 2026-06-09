import type { AdminKeyStatus, AdminProviderSummary } from "../../types/admin";

const stateColors: Record<string, string> = {
  green: "#16a34a",
  red: "#dc2626",
  gold: "#d97706",
  blue: "#2563eb",
  gray: "#64748b",
};

export function renderProviderState(provider: AdminProviderSummary): JSX.Element {
  if (provider.active) {
    return <StateText color="green">当前活跃</StateText>;
  }
  if (provider.active_keys === 0 && provider.cooling_keys === 0 && provider.invalid_keys === 0 && provider.disabled_keys > 0) {
    return <StateText color="gray">已停用</StateText>;
  }
  if (provider.active_keys === 0 && provider.cooling_keys === 0) {
    return <StateText color="red">不可用</StateText>;
  }
  if (provider.cooling_keys > 0 || provider.invalid_keys > 0) {
    return <StateText color="gold">波动中</StateText>;
  }
  if (provider.disabled_keys > 0) {
    return <StateText color="gray">含停用</StateText>;
  }
  return <StateText color="blue">待命</StateText>;
}

export function renderKeyState(state: AdminKeyStatus["state"]): JSX.Element {
  switch (state) {
    case "active":
      return <StateText color="green">可用</StateText>;
    case "cooling":
      return <StateText color="gold">冷却中</StateText>;
    case "disabled":
      return <StateText color="gray">停用</StateText>;
    default:
      return <StateText color="red">失效</StateText>;
  }
}

export function splitLinesText(input: string): string[] {
  return input
    .split(/\r?\n/g)
    .map((value) => value.trim())
    .filter((value) => value.length > 0);
}

export function StateText({ color, children }: { color: string; children: React.ReactNode }): JSX.Element {
  return <span style={{ color: stateColors[color] ?? "#475569", fontWeight: 600, fontSize: "0.82rem" }}>{children}</span>;
}
