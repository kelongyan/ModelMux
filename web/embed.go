package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// distFiles 保存前端构建产物，供生产模式下直接嵌入 Go 二进制。
//
//go:embed all:dist
var distFiles embed.FS

var (
	// staticFiles 提前裁剪到 dist 根目录，避免每次请求重复做 fs.Sub。
	staticFiles = mustSubFS(distFiles, "dist")
	// staticFileServer 统一负责静态资源响应头和文件分发。
	staticFileServer = http.FileServer(http.FS(staticFiles))
)

// NewHandler 返回一个支持 SPA 路由回退的静态资源处理器。
func NewHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := normalizeRequestPath(r.URL.Path)
		if requestPath == "/" {
			serveStaticPath(w, r, "/")
			return
		}
		if hasFileExtension(requestPath) || staticFileExists(strings.TrimPrefix(requestPath, "/")) {
			serveStaticPath(w, r, requestPath)
			return
		}
		serveStaticPath(w, r, "/")
	})
}

// mustSubFS 从嵌入文件系统中提取子目录；阶段 0 启动时若产物缺失应立即失败。
func mustSubFS(root fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(root, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// serveStaticPath 复用标准文件服务，同时允许在回退时改写请求路径。
func serveStaticPath(w http.ResponseWriter, r *http.Request, targetPath string) {
	cloned := r.Clone(r.Context())
	clonedURL := *cloned.URL
	cloned.URL = &clonedURL
	cloned.URL.Path = targetPath
	staticFileServer.ServeHTTP(w, cloned)
}

// normalizeRequestPath 把浏览器请求路径规范化为以斜杠开头的安全路径。
func normalizeRequestPath(rawPath string) string {
	cleaned := path.Clean("/" + rawPath)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

// hasFileExtension 判断请求是否显式指向某个静态文件资源。
func hasFileExtension(requestPath string) bool {
	return strings.Contains(path.Base(requestPath), ".")
}

// staticFileExists 判断 dist 中是否存在对应文件，避免把真实文件误回退到 index。
func staticFileExists(name string) bool {
	if name == "" {
		name = "index.html"
	}
	info, err := fs.Stat(staticFiles, name)
	return err == nil && !info.IsDir()
}
