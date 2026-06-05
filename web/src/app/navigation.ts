export type NavigationItem = {
  key: string;
  label: string;
};

export const navigationItems: NavigationItem[] = [
  { key: "/dashboard", label: "总览" },
  { key: "/providers", label: "提供商" },
  { key: "/stats", label: "调用统计" },
  { key: "/settings", label: "设置" },
  { key: "/events", label: "事件" },
  { key: "/about", label: "关于" },
];
