export type NavigationItem = {
  key: string;
  label: string;
  icon: NavigationIconName;
};

export type NavigationIconName =
  | "dashboard"
  | "providers"
  | "stats"
  | "settings"
  | "events"
  | "about";

export const navigationItems: NavigationItem[] = [
  { key: "/dashboard", label: "总览", icon: "dashboard" },
  { key: "/providers", label: "提供商", icon: "providers" },
  { key: "/stats", label: "调用统计", icon: "stats" },
  { key: "/settings", label: "设置", icon: "settings" },
  { key: "/events", label: "事件", icon: "events" },
  { key: "/about", label: "关于", icon: "about" },
];
