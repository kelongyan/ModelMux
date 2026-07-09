import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter } from "react-router-dom";
import "@fontsource-variable/inter";
import "@fontsource-variable/space-grotesk";
import "@fontsource-variable/jetbrains-mono";

import { App } from "./App";
import { queryClient } from "./app/query-client";
import { AppThemeProvider } from "./app/theme-mode";
import "./styles.css";

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("missing #root element");
}

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <AppThemeProvider>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter basename={normalizeBaseName(import.meta.env.BASE_URL)}>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </AppThemeProvider>
  </React.StrictMode>,
);

function normalizeBaseName(baseUrl: string): string {
  if (!baseUrl || baseUrl === "/") {
    return "/";
  }
  return baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
}
