package web

import (
	"net/http"
	"path/filepath"
)

// IndexHandler 处理除/api/以外的所有HTTP请求
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	// 获取请求的文件路径
	path := r.URL.Path

	// 如果请求的是根目录，则重定向到默认的index文件
	if path == "/" {
		path = "/index.html"
	}

	// 获取文件的真实路径，这里假设网站的内容都放在名为"static"的目录下
	// 请根据实际情况修改这个路径
	staticFilePath := filepath.Join("public", path)

	// 使用http.ServeFile来响应文件请求
	http.ServeFile(w, r, staticFilePath)
}
