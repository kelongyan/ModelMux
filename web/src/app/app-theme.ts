import { theme, type ThemeConfig } from "antd";

export type AppThemeMode = "light" | "dark";

export function createAppTheme(mode: AppThemeMode): ThemeConfig {
  const dark = mode === "dark";

  return {
    algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      /* === Anthropic Warm Color Palette === */
      colorPrimary: "#cc785c",
      colorInfo: "#cc785c",
      colorSuccess: "#5db872",
      colorWarning: "#d4a017",
      colorError: "#c64545",
      colorText: dark ? "#f0ece6" : "#141413",
      colorTextSecondary: dark ? "#d4cfc7" : "#3d3d3a",
      colorBorder: dark ? "#2d2b28" : "#e6dfd8",
      colorBorderSecondary: dark ? "#2d2b28" : "#e6dfd8",
      colorBgBase: dark ? "#181715" : "#faf9f5",
      colorBgLayout: dark ? "#181715" : "#faf9f5",
      colorBgContainer: dark ? "#252320" : "#faf9f5",

      /* === Anthropic Rounded Corners === */
      borderRadius: 12,
      borderRadiusLG: 16,
      borderRadiusSM: 8,
      borderRadiusXS: 6,

      /* === Controls === */
      controlHeight: 42,
      controlHeightSM: 34,
      controlHeightLG: 48,

      /* === Typography === */
      fontSize: 15,
      fontSizeSM: 13,
      lineHeight: 1.62,
      padding: 18,
      paddingSM: 14,
      paddingXS: 10,
      margin: 18,
      marginSM: 14,
      marginXS: 10,
      fontFamily:
        '"Inter Variable", "PingFang SC", "Microsoft YaHei UI", "Noto Sans SC", system-ui, sans-serif',
      fontFamilyCode:
        '"Cascadia Code", "JetBrains Mono", "SF Mono", Menlo, Consolas, monospace',
    },
    components: {
      Button: {
        primaryShadow: dark
          ? "none"
          : "0 4px 16px rgba(204, 120, 92, 0.25)",
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
