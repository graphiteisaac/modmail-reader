package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

type Message struct {
	Original string
	Content  template.HTML
	Edits    []template.HTML
}

type Token struct {
	Time     string
	Type     string
	User     string
	Role     string
	Color    string
	Messages []Message
}

func processMessage(content string) template.HTML {

	content = regexp.MustCompile(`(?:www|https?)[^(\s\n>)]+`).ReplaceAllStringFunc(content, func(str string) string {
		parsed, err := url.Parse(str)

		if err == nil {
			// is (probably) a video
			if func() bool {
				return strings.HasSuffix(parsed.Path, ".mov") ||
					strings.HasSuffix(parsed.Path, ".mp4") ||
					strings.HasSuffix(parsed.Path, ".avi") ||
					strings.HasSuffix(parsed.Path, ".flv")
			}() {
				return ` <div class="video"><video controls><source src="` + str + `" /></video></div> `
			}

			// is an image
			if func() bool {
				return strings.HasSuffix(parsed.Path, ".png") ||
					strings.HasSuffix(parsed.Path, ".jpeg") ||
					strings.HasSuffix(parsed.Path, ".jpg") ||
					strings.HasSuffix(parsed.Path, ".webp") ||
					strings.HasSuffix(parsed.Path, ".gif")
			}() {
				return ` <div class="img"><img src="` + str + `" /></div> `
			}
		} else {
			fmt.Println(err)
		}

		return ""
	})

	bold := regexp.MustCompile(`(\*\*)(.+?)(\*\*)`)
	italic := regexp.MustCompile(`(\*)(.+?)(\*)`)
	boldItalic := regexp.MustCompile(`(\*\*\*)(.+?)(\*\*\*)`)
	code := regexp.MustCompile("(`+)(.+?)(`+)")

	content = strings.ReplaceAll(content, "\n", " <br/> ")
	content = bold.ReplaceAllString(content, "<b>$2</b>")
	content = italic.ReplaceAllString(content, "<i>$2</i>")
	content = boldItalic.ReplaceAllString(content, "<b><i>$2</i></b>")
	content = code.ReplaceAllString(content, "<code>$2</code>")

	return template.HTML(content)
}

func main() {
	port := flag.Int("host-port", 8087, "Port for the application - defaults to 8086.")
	flag.Parse()

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
		}
	}()

	mux := http.NewServeMux()

	views, err := template.New("app").ParseGlob("view/*.gohtml")
	if err != nil {
		log.Fatal(err)
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(map[string]string{"Status": "OK"})
		w.Write(b)
	})

	mux.HandleFunc("/read", func(w http.ResponseWriter, r *http.Request) {
		modmailURL := r.FormValue("url")
		resp, err := http.Get(modmailURL)
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
		rawContent := []rune(split[1] + "\n[2006-01-02 15:04:05] [END] End of ModMail\n\n")

		tokens := *new([]*Token)
		row := 0
		block := 0
		buf := *new([]rune)
		token := new(Token)

		for i, c := range rawContent {
			switch c {
			case '[':
				continue
			case ']':
				if string(buf) == "REPLY DELETED" {
					// TODO parse the deleted message and place it in chat, maybe with a red background
				}
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
				if i+1 < len(rawContent) && rawContent[i+1] == '[' {
					row++
					block = 0

					content := string(buf)
					msg := Message{
						Content:  template.HTML(""),
						Edits:    []template.HTML{},
						Original: string(buf)[1:],
					}

					if strings.Contains(content, "The user edited their message") {
						afterPos := strings.Index(content, "`A:` ")
						beforePos := strings.Index(content, "`B:` ")

						beforeContent := content[beforePos+len("`B:` ") : afterPos-1]
						for _, str := range regexp.MustCompile(`(?:www|https?)[^(\s|\n)]+`).FindAllString(beforeContent, -1) {
							strings.ReplaceAll(beforeContent, str, "<"+str+">")
						}

						for _, t := range tokens {
							for _, m := range t.Messages {
								if beforeContent == m.Original {
									m.Edits = append(m.Edits, template.HTML(content))
									//m.Content = processMessage(content[afterPos+len("`A:` "):])
								}
							}
						}

						continue
					}

					if token.Type == "TO USER" {
						role := regexp.MustCompile(`^ \(.*?\) `).FindString(content)
						token.Role = role[2 : len(role)-2]

						content = content[strings.Index(content, token.User+":")+len(token.User+": "):]
					}

					tokenIndex := len(tokens) - 1
					if tokenIndex < 0 {
						tokenIndex = 0
					}

					msg.Content = processMessage(content)

					if len(tokens) > 0 && tokens[tokenIndex].User == token.User {
						tokens[tokenIndex].Messages = append(tokens[len(tokens)-1].Messages, msg)
					} else {
						token.Messages = append(token.Messages, msg)
						tokens = append(tokens, token)
					}

					token = &Token{}
					buf = []rune{}
				} else {
					buf = append(buf, '\n')
				}
			default:
				if i+1 < len(rawContent) && rawContent[i+1] != '[' {
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

	s := http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", *port), Handler: mux}

	fmt.Printf("Server started http://localhost:%d\n", *port)
	s.ListenAndServe()
}
