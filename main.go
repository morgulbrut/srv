package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
)

type context struct {
	srvDir string
}

func renderListing(w http.ResponseWriter, r *http.Request, f *os.File) error {
	files, err := f.Readdir(-1)
	if err != nil {
		return err
	}
    fmt.Fprintf(w, "<style>html { font-family: monospace; } table { border: none; margin: 1rem; } td { padding-right: 2rem; }</style>\n")
    fmt.Fprintf(w, "<table>")
	for _, fi := range files {
        fmt.Fprintf(w, "<tr>")
        // TODO: separately sort hidden files? probably an optional feature, would make code more complicated
		name, size := fi.Name(), fi.Size()
        path := path.Join(r.URL.Path, name)
		switch {
        // TODO: css ellipsis e.g. text-overflow: ellipsis;
		case fi.IsDir():
			fmt.Fprintf(w, "<td><a href=\"%s/\">%s/</a></td>\n", path, name)
		case ! fi.Mode().IsRegular():
			fmt.Fprintf(w, "<td><p style=\"color: #777\">%s</p></td>\n", name)
		default:
			fmt.Fprintf(w, "<td><a href=\"%s\">%s</a></td><td>%d</td>\n", path, name, size)
		}
        fmt.Fprintf(w, "</tr>")
	}
    fmt.Fprintf(w, "</table>")
	return nil
}

func (c *context) handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// path.Join is Cleaned, but docstring for http.ServeFile says joining r.URL.Path isn't safe
		// however this seems fine? might want to add a small test suite with some dir traversal attacks
		fp := path.Join(c.srvDir, r.URL.Path)

        fi, err := os.Lstat(fp)
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		f, err := os.OpenFile(fp, os.O_RDONLY, 0444)
		defer f.Close()
		if err != nil {
			http.Error(w, "failed to open file", http.StatusInternalServerError)
			return
		}

		switch {
		case fi.IsDir():
			// XXX: if a symlink has name "index.html", it will be served here.
			// i could add an extra lstat here, but the scenario is just too rare to justify the additional file operation.
            html, err := os.OpenFile(path.Join(fp, "index.html"), os.O_RDONLY, 0444)
            defer html.Close()
            if err == nil {
                io.Copy(w, html)
                return
            }
			err = renderListing(w, r, f)
			if err != nil {
				http.Error(w, "failed to render directory listing: "+err.Error(), http.StatusInternalServerError)
			}
		case fi.Mode().IsRegular():
			io.Copy(w, f)
		default:
			http.Error(w, "file isn't a regular file or directory", http.StatusForbidden)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func die(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
	os.Stderr.Write([]byte("\n"))
	os.Exit(1)
}

func main() {
	argv := len(os.Args)
	var srvDir string
	switch {
	case argv == 3:
		srvDir = os.Args[2]
		f, err := os.Open(srvDir)
		defer f.Close()
		if err != nil {
			die(err.Error())
		}
		if fi, err := f.Stat(); err != nil || !fi.IsDir() {
			die("%s isn't a directory", srvDir)
		}
	case argv == 2:
		var exists bool
		srvDir, exists = os.LookupEnv("PWD")
		if !exists {
			die("PWD is not set, cannot infer directory.")
		}
	default:
		die(`srv ver. %s

usage: %s port [directory]

directory	path to directory to serve (default: PWD)
`, "0.0", os.Args[0])
	}
	port := os.Args[1]

	c := &context{
		srvDir: srvDir,
	}
    http.HandleFunc("/", c.handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}