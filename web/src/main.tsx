import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConfigProvider } from "antd";
import { BrowserRouter } from "react-router-dom";

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
          colorPrimary: "#0f766e",
          colorInfo: "#0f766e",
          colorSuccess: "#15803d",
          colorWarning: "#b54708",
          colorError: "#b42318",
          colorText: "#0f172a",
          colorTextSecondary: "#475467",
          colorBorder: "#d0d5dd",
          colorBorderSecondary: "#e4e7ec",
          colorBgBase: "#f5f7fa",
          colorBgLayout: "#f5f7fa",
          colorBgContainer: "#ffffff",
          borderRadius: 12,
          borderRadiusLG: 16,
          borderRadiusSM: 10,
          borderRadiusXS: 8,
          controlHeight: 38,
          controlHeightSM: 30,
          controlHeightLG: 44,
          fontFamily: '"Aptos", "Segoe UI", "Microsoft YaHei UI", sans-serif',
        },
        components: {
          Button: {
            primaryShadow: "0 2px 6px rgba(15, 118, 110, 0.14)",
          },
          Tag: {
            borderRadiusSM: 999,
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
