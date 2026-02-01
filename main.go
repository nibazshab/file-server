package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/webdav"
)

type httpHandler struct {
	root *os.Root
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	if path == "" {
		path = "."
	}

	fi, err := h.root.Stat(path)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	f, _ := h.root.Open(path)
	defer f.Close()

	if !fi.IsDir() {
		http.ServeContent(w, req, fi.Name(), fi.ModTime(), f)
		return
	}

	entries, _ := f.ReadDir(-1)
	slices.SortStableFunc(entries, func(a, b os.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	})

	padding := make([]byte, 51)
	for i := range padding {
		padding[i] = ' '
	}

	var b strings.Builder

	top := "Index of " + req.URL.Path
	fmt.Fprintf(&b, "<title>%s</title><h1>%s</h1><hr>", top, top)
	b.WriteString("<pre><a href=\"../\">../</a>\n")

	for _, entry := range entries {
		var i, j int
		var name, size, time string

		e, _ := entry.Info()

		if e.IsDir() {
			name = e.Name() + "/"
			size = "-"
		} else {
			name = e.Name()
			size = strconv.FormatInt(e.Size(), 10)
		}

		time = e.ModTime().Format("02-Jan-2006 15:04")

		i = max(51-len(name), 1)
		j = max(20-len(size), 1)

		fmt.Fprintf(&b, "<a href=\"%s\">%s</a>", name, name)
		b.Write(padding[:i])
		b.WriteString(time)
		b.Write(padding[:j])
		b.WriteString(size)
		b.WriteString("\n")
	}

	b.WriteString("</pre><hr>")
	w.Write([]byte(b.String()))
}

type webdavFs struct {
	webdav.FileSystem
}

func (wd webdavFs) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return os.ErrPermission
}

func (wd webdavFs) RemoveAll(ctx context.Context, name string) error {
	return os.ErrPermission
}

func (wd webdavFs) Rename(ctx context.Context, oldName, newName string) error {
	return os.ErrPermission
}

func (wd webdavFs) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, os.ErrPermission
	}

	return wd.FileSystem.OpenFile(ctx, name, flag, perm)
}

func logMiddleware(tag string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[%s] %s %s from %s\n", tag, r.Method, r.URL.Path, r.RemoteAddr)
		h.ServeHTTP(w, r)
	})
}

func main() {
	var port, path, davPort string

	flag.StringVar(&davPort, "dav-port", "8081", "webdav server port")
	flag.StringVar(&port, "port", "8080", "http server port")
	flag.StringVar(&path, "path", "./", "server path")
	flag.Parse()

	path, _ = filepath.Abs(path)
	fmt.Printf("PATH: %s\n", path)

	go func() {
		davHandler := &webdav.Handler{
			FileSystem: webdavFs{
				FileSystem: webdav.Dir(path),
			},
			LockSystem: webdav.NewMemLS(),
		}

		hdr := logMiddleware("WEBDAV", davHandler)

		fmt.Printf("WebDAV: http://[ipv4/ipv6]:%s\n", davPort)
		http.ListenAndServe(":"+davPort, hdr)
	}()

	{
		httpFs, _ := os.OpenRoot(path)
		httpMux := http.NewServeMux()
		httpMux.Handle("/", &httpHandler{root: httpFs})

		hdr := logMiddleware("HTTP", httpMux)

		fmt.Printf("HTTP: http://[ipv4/ipv6]:%s\n", port)
		http.ListenAndServe(":"+port, hdr)
	}
}
