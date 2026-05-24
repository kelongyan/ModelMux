package admin

import (
	"net/http"

	webui "github.com/kelongyan/ModelMux/web"
)

// registerConsoleRoutes 注册管理台页面入口。
func (h *Handler) registerConsoleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/console", h.consoleRootRedirect)
	mux.Handle("/console/", http.StripPrefix("/console", webui.NewHandler()))
}

// consoleRootRedirect 把不带斜杠的 console 根路径规范化到 SPA 入口。
func (h *Handler) consoleRootRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/console/", http.StatusTemporaryRedirect)
}
