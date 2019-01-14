package main

import (
	"crypto/sha1"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
)

type Page struct {
	Title string
	Body  template.HTML
	Markdown []byte
	User string
}

func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Markdown, 0600)
}

func renderMarkdown(title string, input []byte) []byte {
	extensions := 0
	extensions |= blackfriday.EXTENSION_NO_INTRA_EMPHASIS
	extensions |= blackfriday.EXTENSION_TABLES
	extensions |= blackfriday.EXTENSION_FENCED_CODE
	extensions |= blackfriday.EXTENSION_AUTOLINK
	extensions |= blackfriday.EXTENSION_STRIKETHROUGH
	extensions |= blackfriday.EXTENSION_SPACE_HEADERS

	htmlFlags := 0
	htmlFlags |= blackfriday.HTML_TOC
	htmlFlags |= blackfriday.HTML_COMPLETE_PAGE

	renderer := blackfriday.HtmlRenderer(htmlFlags, title, "")

	return blackfriday.Markdown(input, renderer, extensions)
}

func loadPage(title string, user string) (*Page, error) {
	filename := "data/" + title + ".txt"
	markdown, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	unsafe := renderMarkdown(title, markdown)
	body := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	return &Page{Title: title, User: user,
		Markdown: markdown, Body: template.HTML(body)}, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	p, err := loadPage(title, user)
	if err != nil {
		http.Redirect(w, r, "/edit/" + title, http.StatusFound)
		return
	}
	renderPage(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	p, err := loadPage(title, user)
	if err != nil {
		p = &Page{Title: title}
	}
	renderPage(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	markdown := r.FormValue("markdown")
	p := &Page{Title: title, Markdown: []byte(markdown)}

	os.MkdirAll("data", os.ModePerm)
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/" + title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles("index.html", "edit.html", "view.html"))

func indexHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	files, err := ioutil.ReadDir("data")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var pages []string
	for _, v := range files {
		pages = append(pages, strings.Split(v.Name(), ".")[0])
	}

	err = templates.ExecuteTemplate(w, "index.html", pages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPage(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl + ".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getUser(r *http.Request) string {
	addr := strings.Split(r.RemoteAddr, ":")[0]
	return fmt.Sprintf("%x", sha1.Sum([]byte(addr)))
}

var validPath = regexp.MustCompile("^/(index|edit|save|view)/([a-zA-Z0-9-]*)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Redirect(w, r, "/index/", http.StatusFound)
			return
		}
		user := getUser(r)
		fn(w, r, m[2], user)
	}
}

func redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/index/", http.StatusFound)
}

func main() {
	http.HandleFunc("/", redirect)
	http.HandleFunc("/index/", makeHandler(indexHandler))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

