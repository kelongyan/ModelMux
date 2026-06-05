import { theme, type ThemeConfig } from "antd";

export type AppThemeMode = "light" | "dark";

export function createAppTheme(mode: AppThemeMode): ThemeConfig {
  const dark = mode === "dark";

  return {
    algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary: "#1677ff",
      colorInfo: "#1677ff",
      colorSuccess: dark ? "#22c55e" : "#16a34a",
      colorWarning: dark ? "#fbbf24" : "#f59e0b",
      colorError: dark ? "#f87171" : "#ef4444",
      colorText: dark ? "#e5edf7" : "#0f172a",
      colorTextSecondary: dark ? "#94a3b8" : "#64748b",
      colorBorder: dark ? "#22324a" : "#dbeafe",
      colorBorderSecondary: dark ? "#1e2d44" : "#e0edff",
      colorBgBase: dark ? "#07111f" : "#f5f9ff",
      colorBgLayout: dark ? "#07111f" : "#f5f9ff",
      colorBgContainer: dark ? "#0f1b2d" : "#ffffff",
      borderRadius: 14,
      borderRadiusLG: 18,
      borderRadiusSM: 10,
      borderRadiusXS: 8,
      controlHeight: 42,
      controlHeightSM: 34,
      controlHeightLG: 48,
      fontSize: 15,
      fontSizeSM: 13,
      lineHeight: 1.62,
      padding: 18,
      paddingSM: 14,
      paddingXS: 10,
      margin: 18,
      marginSM: 14,
      marginXS: 10,
      fontFamily: '"Inter Variable", "PingFang SC", "Microsoft YaHei UI", "Noto Sans SC", system-ui, sans-serif',
      fontFamilyCode: '"Cascadia Code", "JetBrains Mono", "SF Mono", Menlo, Consolas, monospace',
    },
    components: {
      Button: {
        primaryShadow: dark ? "none" : "0 8px 18px rgba(22, 119, 255, 0.2)",
        paddingInline: 18,
      },
      Tag: {
        borderRadiusSM: 999,
        fontSizeSM: 13,
        lineHeightSM: 1.8,
      },
    },
  };
}
