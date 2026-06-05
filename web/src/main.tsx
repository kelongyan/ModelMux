import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConfigProvider } from "antd";
import { BrowserRouter } from "react-router-dom";
import "@fontsource-variable/inter";

import { App } from "./App";
import "./styles.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
    },
  },
});

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("missing #root element");
}

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#1677ff",
          colorInfo: "#1677ff",
          colorSuccess: "#16a34a",
          colorWarning: "#f59e0b",
          colorError: "#ef4444",
          colorText: "#0f172a",
          colorTextSecondary: "#64748b",
          colorBorder: "#dbeafe",
          colorBorderSecondary: "#e0edff",
          colorBgBase: "#f5f9ff",
          colorBgLayout: "#f5f9ff",
          colorBgContainer: "#ffffff",
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
            primaryShadow: "0 8px 18px rgba(22, 119, 255, 0.2)",
            paddingInline: 18,
          },
          Tag: {
            borderRadiusSM: 999,
            fontSizeSM: 13,
            lineHeightSM: 1.8,
          },
        },
      }}
    >
      <QueryClientProvider client={queryClient}>
        <BrowserRouter basename={normalizeBaseName(import.meta.env.BASE_URL)}>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </ConfigProvider>
  </React.StrictMode>
);

function normalizeBaseName(baseUrl: string): string {
  if (!baseUrl || baseUrl === "/") {
    return "/";
  }
  return baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
}
