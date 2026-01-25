package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type fileHandler struct {
	root *os.Root
}

func (fh *fileHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("%s\n", req.URL.Path)

	path := strings.TrimPrefix(req.URL.Path, "/")
	if path == "" {
		path = "."
	}

	fi, err := fh.root.Stat(path)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	f, _ := fh.root.Open(path)
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
	b.WriteString(fmt.Sprintf("<title>%s</title><h1>%s</h1><hr>", top, top))
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

		b.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a>", name, name))
		b.Write(padding[:i])
		b.WriteString(time)
		b.Write(padding[:j])
		b.WriteString(size)
		b.WriteString("\n")
	}

	b.WriteString("</pre><hr>")
	w.Write([]byte(b.String())) 
}

func main() {
	var port, path string

	flag.StringVar(&port, "port", "8080", "server port")
    flag.StringVar(&path, "path", "./", "server path")
	flag.Parse()

	path, _ = filepath.Abs(path)
	rootfs, _ := os.OpenRoot(path)

	fmt.Printf("ipv4/[ipv6]:%s, %s\n", port, path)

	http.Handle("/", &fileHandler{root: rootfs})
	http.ListenAndServe(":"+port, nil)
}
