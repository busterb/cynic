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
	Assessment string
	Reaction string
	Body template.HTML
}

type Page struct {
	Title string
	Body  template.HTML
	Markdown []byte
	User string
	CurrentTopics []string
	OldTopics []string
	Comments []Comment
}

func pageFile(title string, user string, mode string) string {
	if mode == "edit" {
		return "data/" + title + ".md"
	} else if mode == "comment" || mode == "assessment" {
		return "data/" + title + "_" + mode + "_" + user + ".md"
	}
	return ""
}

func (p *Page) save(mode string, assessment string) error {
	if mode != "edit" {
		if assessment != "" {
			assessmentFile := pageFile(p.Title, p.User, "assessment")
			ioutil.WriteFile(assessmentFile, []byte(assessment), 0600)
		}
	}
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

func userCommented(title string, user string) bool {
	name := pageFile(title, user, "comment")
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
    }
    return true
}

func userAssessment(title string, user string) (string, error) {
	name := pageFile(title, user, "assessment")
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return "Shrug", nil
		}
    }
	assessment, err := ioutil.ReadFile(name)
	return string(assessment), err
}


func viewHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	p, err := loadPage(title, user, "edit")
	renderTopicComments(p)
	if err != nil {
		http.Redirect(w, r, "/edit/" + title, http.StatusFound)
		return
	}

	markdown, err := ioutil.ReadFile(pageFile(p.Title, p.User, "comment"))
	if err == nil {
		p.Markdown = markdown
	} else {
		p.Markdown = nil
	}

	renderPage(w, "view", p)
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
	assessment := r.FormValue("assessment")
	next := r.FormValue("next")

	p := &Page{Title: title, User: user, Markdown: []byte(markdown)}

	os.MkdirAll("data", os.ModePerm)
	if !(assessment == "" && markdown == "") {
		err := p.save(mode, assessment)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if next == "Next" {
		current, _, err := getTopics(user)
		foundIndex := -1
		nextIndex := -1
		if (err == nil && len(current) > 0) {
			for idx, topic := range current {
				if topic == title {
					foundIndex = idx
					break
				}
			}
			if foundIndex == len(current) - 1 {
				nextIndex = 0
			} else if (foundIndex < len(current) - 1) {
				nextIndex = foundIndex + 1
			}
		}
		if nextIndex != -1 {
			http.Redirect(w, r, "/view/" + current[nextIndex], http.StatusFound)
			return
		}
		http.Redirect(w, r, "/topics/", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/view/" + title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles("topics.html", "edit.html", "view.html"))

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
				assessment, _ := userAssessment(p.Title, user)
				reaction := "maybe.png"
				if assessment == "Hot" {
					reaction = "yes.gif"
				} else if assessment == "Not" {
					reaction = "no.png"
				} else {
					reaction = "shrug.jpg"
				}

				p.Comments = append(p.Comments,
					Comment{User: user, Assessment: assessment, Reaction: reaction,
						Body: template.HTML(html)})
			} else {
				log.Printf("%s", err.Error())
			}
		}
	}
	return nil
}

func getTopics(user string) (current []string, old []string, err error) {
	files, err := ioutil.ReadDir("data")
	if err != nil {
		return nil, nil, err
	}

	for _, v := range files {
		name := v.Name()
		if !strings.Contains(name, "_") {
			topic := strings.Split(v.Name(), ".")[0]
			if !userCommented(topic, user) {
				current = append(current, topic)
			} else {
				old = append(old, topic)
			}
		}
	}
	return current, old, nil
}

func topicsHandler(w http.ResponseWriter, r *http.Request, title string, user string) {
	p := &Page{Title: "Topics", User: user}
	current, old, err := getTopics(p.User)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p.CurrentTopics = current
	p.OldTopics = old
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

var validPath = regexp.MustCompile("^/(images|topics|edit|save|view)/([a-zA-Z0-9-]*)$")

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
	fileServer := http.FileServer(http.Dir("images"))
	http.Handle("/images/", http.StripPrefix("/images/", fileServer))
	http.HandleFunc("/topics/", makeHandler(topicsHandler))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/new/", newHandler)
	http.HandleFunc("/save/", makeHandler(saveHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

