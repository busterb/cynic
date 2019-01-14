package main

import (
	"crypto/sha1"
	"errors"
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

type Comment struct {
	User string
	Body template.HTML
}

type Page struct {
	Title string
	Body  template.HTML
	Markdown []byte
	User string
	NewTopics []string
	OldTopics []string
	Comments []Comment
}

func pageFile(title string, user string, mode string) string {
	if mode == "edit" {
		return "data/" + title + ".md"
	} else if mode == "comment" {
		return "data/" + title + "_" + mode + "_" + user + ".md"
	}
	return ""
}

func (p *Page) save(mode string) error {
	filename := pageFile(p.Title, p.User, mode)
	if filename == "" {
		return errors.New("invalid mode")
	}
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

func mdToHtml(filename string, title string) ([]byte, []byte, error) {
	markdown, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	unsafe := renderMarkdown(title, markdown)
	html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	return markdown, html, nil
}

func loadPage(title string, user string, mode string) (*Page, error) {
	filename := pageFile(title, user, mode)
	markdown, html, err := mdToHtml(filename, title)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, User: user, Markdown: markdown, Body: template.HTML(html)}, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	p, err := loadPage(title, user, "edit")
	renderTopicComments(p)
	if err != nil {
		http.Redirect(w, r, "/edit/" + title, http.StatusFound)
		return
	}
	renderPage(w, "view", p)
}

func commentHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	p, err := loadPage(title, user, "comment")
	if err != nil {
		p = &Page{Title: title, User: user}
	}
	renderPage(w, "comment", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	p, err := loadPage(title, user, "edit")
	if err != nil {
		p = &Page{Title: title, User: user}
	}
	renderPage(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	markdown := r.FormValue("markdown")
	mode := r.FormValue("mode")
	p := &Page{Title: title, User: user, Markdown: []byte(markdown)}

	os.MkdirAll("data", os.ModePerm)
	err := p.save(mode)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/" + title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles("topics.html", "comment.html", "edit.html", "view.html"))

func renderTopicComments(p *Page) error {
	files, err := ioutil.ReadDir("data")
	if err != nil {
		return errors.New("could not read data directory")
	}

	for _, v := range files {
		filename := v.Name()
		if strings.Contains(filename, p.Title + "_comment_") {
			user := strings.Split(strings.Split(filename, "_comment_")[1], ".")[0]
			_, html, err := mdToHtml("data/" + filename, p.Title)
			if err == nil {
				log.Printf("found comment " + filename)
				p.Comments = append(p.Comments, Comment{User: user, Body: template.HTML(html)})
			} else {
				log.Printf("%s", err.Error())
			}
		}
	}
	return nil
}

func topicsHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	files, err := ioutil.ReadDir("data")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p := &Page{Title: "Topics", User: user}
	for _, v := range files {
		name := v.Name()
		if !strings.Contains(name, "_comment_") {
			p.NewTopics = append(p.NewTopics, strings.Split(v.Name(), ".")[0])
		}
	}

	renderPage(w, "topics", p)
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

var validPath = regexp.MustCompile("^/(topics|comment|edit|save|view|upvote|downvote)/([a-zA-Z0-9-]*)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Redirect(w, r, "/topics/", http.StatusFound)
			return
		}
		user := getUser(r)
		fn(w, r, m[2], user)
	}
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/topics/", http.StatusFound)
}

func newHandler(w http.ResponseWriter, r *http.Request) {
	topic := r.FormValue("topic")
	http.Redirect(w, r, "/edit/" + topic, http.StatusFound)
}

func main() {
	http.HandleFunc("/", redirectHandler)
	http.HandleFunc("/topics/", makeHandler(topicsHandler))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/comment/", makeHandler(commentHandler))
	http.HandleFunc("/new/", newHandler)
	/*
	http.HandleFunc("/upvote/", makeHandler(upvoteHandler))
	http.HandleFunc("/downvote/", makeHandler(downvoteHandler))
	*/
	http.HandleFunc("/save/", makeHandler(saveHandler))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

