package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func main() {
	mux := http.NewServeMux()

	views, err := template.New("app").ParseGlob("view/*.gohtml")
	if err != nil {
		log.Fatal(err)
	}

	mux.HandleFunc("/read", func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("url")

		resp, err := http.Get(url)
		if err != nil {
			fmt.Println(err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
		}

		if !strings.HasPrefix(string(body), "# Modmail thread") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<h2 class="error">The provided URL is not a ModMail thread.</h2>`))
			return
		}

		split := strings.Split(string(body), "\n────────────────\n")
		//head := split[0]
		content := []rune(split[1] + "\n[2006-01-02 15:04:05] [END] End of ModMail\n\n")

		type Token struct {
			Time    string
			Type    string
			User    string
			Content template.HTML
		}

		tokens := []*Token{}

		row := 0
		block := 0
		buf := []rune{}
		token := &Token{}

		for i, c := range content {
			switch c {
			case '[':
				continue
			case ']':
				if block == 0 {
					t, err := time.Parse("2006-01-02 15:04:05", string(buf))
					token.Time = t.Format(time.Stamp)
					if err != nil {
						fmt.Println(err)
					}
				} else if block == 1 {
					token.Type = string(buf)
				} else if block == 2 {
					token.User = string(buf)
				}
				buf = []rune{}
				block++
			case '\n':
				if i+1 < len(content) && content[i+1] == '[' {
					row++
					block = 0

					if strings.Contains(string(buf), "The user edited their message") {

					}

					bold := regexp.MustCompile(`(\*\*)(.+?)(\*\*)`)
					italic := regexp.MustCompile(`(\*)(.+?)(\*)`)
					boldItalic := regexp.MustCompile(`(\*\*\*)(.+?)(\*\*\*)`)
					code := regexp.MustCompile("(`+)(.+?)(`+)")

					cnt := bold.ReplaceAllString(string(buf), "<b>$2</b>")
					cnt = italic.ReplaceAllString(cnt, "<i>$2</i>")
					cnt = boldItalic.ReplaceAllString(cnt, "<b><i>$2</i></b>")
					cnt = strings.ReplaceAll(cnt, "`B:`", "<code class=\"before\">Before:</code>")
					cnt = strings.ReplaceAll(cnt, "`A:`", "<code class=\"after\">After:</code>")
					cnt = code.ReplaceAllString(cnt, "<code>$2</code>")

					cnt = regexp.MustCompile(`(?i)(https?:\/\/.*\.(?:png|jpg|webp|jpeg|gif))\??(?:&?[^=&]*=[^=&]*)*([^\s]+)`).ReplaceAllString(cnt, "<div class=\"img\"><img src=\"$1\" /></div>")
					cnt = regexp.MustCompile(`(?i)(https?:\/\/.*\.(?:mov|mp4|avi|flv))\??(?:&?[^=&]*=[^=&]*)*([^\s]+)`).ReplaceAllString(cnt, "<div class=\"video\"><video controls> <source src=\"$1\" /></video></div>")

					cnt = strings.ReplaceAll(cnt, "\n", "<br/>")
					token.Content = template.HTML(cnt)
					tokens = append(tokens, token)
					token = &Token{}

					buf = []rune{}
				} else {
					buf = append(buf, '\n')
				}
			default:
				if i+1 < len(content) && content[i+1] != '[' {
					buf = append(buf, c)
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		views.ExecuteTemplate(w, "read.gohtml", map[string][]*Token{"tokens": tokens})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		views.ExecuteTemplate(w, "app.gohtml", nil)
	})

	s := http.Server{Addr: "0.0.0.0:8087", Handler: mux}

	fmt.Println("Server started http://localhost:8087")
	s.ListenAndServe()
}
