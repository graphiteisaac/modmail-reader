package main

import (
	"bytes"
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

func main() {
	port := flag.Int("host-port", 8087, "Port for the application - defaults to 8087.")
	dev := flag.Bool("dev", false, "Development mode - enables debug route")
	flag.Parse()

	// Handle recovering from panics
	defer recovery()

	mux := http.NewServeMux()

	views, err := template.New("app").ParseGlob("view/*.gohtml")
	if err != nil {
		log.Fatal(err)
	}

	c := Ctx{
		views,
	}

	if *dev {
		mux.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
			modmail, err := retrieveModmail(r.FormValue("url"))
			if err != nil {
				_, _ = w.Write([]byte(`<div class="error">` + err.Error() + `</div>`))
				return
			}

			info, tokens, err := parseModmail(modmail)
			if err != nil {
				_, _ = w.Write([]byte(`<div class="error">` + err.Error() + `</div>`))
				return
			}


            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{
				"info":   info,
				"tokens": tokens,
			})
		})
	}

	mux.HandleFunc("/app.css", c.serveCSS)
	mux.HandleFunc("/health", c.healthcheck)
	mux.HandleFunc("/read", c.read)
	mux.HandleFunc("/", c.homepage)

	httpServer := http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", *port), Handler: mux}
	fmt.Printf("Server started http://localhost:%d\n", *port)

	if err := httpServer.ListenAndServe(); err != nil {
		fmt.Println(err)
	}
}

type Message struct {
	Original string
	Content  template.HTML
	Edits    []template.HTML
}

type Token struct {
	Time      string
	Type      string
	User      string
	Role      string
	Color     string
	Anonymous bool
	Messages  []Message
}

type ThreadServer struct {
	ServerName string
	Nickname   string
	Joined     string
	Roles      []string
}

type ThreadInfo struct {
	UserID     string
	Username   string
	AccountAge time.Time
	NumThreads int
	Opened     time.Time
	Servers    []ThreadServer
}

type Ctx struct {
	views *template.Template
}

func (c *Ctx) homepage(w http.ResponseWriter, r *http.Request) {
	var result bytes.Buffer

	if len(r.FormValue("t")) > 0 {
		modmail, err := retrieveModmail(r.FormValue("t"))
		if err != nil {
			// TODO: Handle error
			_, _ = w.Write([]byte(`<div class="error">` + err.Error() + `</div>`))
		}

		info, tokens, err := parseModmail(modmail)
		_ = c.views.ExecuteTemplate(&result, "read.gohtml", map[string]any{
			"info":   info,
			"tokens": tokens,
		})
	}

	_ = c.views.ExecuteTemplate(w, "app.gohtml", map[string]template.HTML{
		"result": template.HTML(result.String()),
	})
}

func (c *Ctx) serveCSS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/app.css")
}

func (c *Ctx) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

func (c *Ctx) read(w http.ResponseWriter, r *http.Request) {
	modmail, err := retrieveModmail(r.FormValue("url"))
	if err != nil {
		_, _ = w.Write([]byte(`<div class="error">` + err.Error() + `</div>`))
		return
	}

	info, tokens, err := parseModmail(modmail)
	if err != nil {
		_, _ = w.Write([]byte(`<div class="error">` + err.Error() + `</div>`))
		return
	}

	_ = c.views.ExecuteTemplate(w, "read.gohtml", map[string]any{
		"info":   info,
		"tokens": tokens,
	})
}

func recovery() {
	if r := recover(); r != nil {
		fmt.Println("Fatal panic stacktrace from panic: \n" + string(debug.Stack()))
	}
}

func processMessage(content string) template.HTML {
    content = strings.TrimSpace(content)

	content = regexp.MustCompile(`(?:www|https?)[^(\s\n>)]+`).ReplaceAllStringFunc(content, func(str string) string {
		parsed, err := url.Parse(str)
		if err != nil {
			return ""
		}

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

		return fmt.Sprintf(`<a href="%s">%s</a>`, str, str)
	})

	// Line breaks
	content = strings.ReplaceAll(content, "\n", " <br/> ")
	// Bold blocks
	content = regexp.MustCompile(`(\*\*)(.+?)(\*\*)`).ReplaceAllString(content, "<b>$2</b>")
	// Italic blocks
	content = regexp.MustCompile(`(\*)(.+?)(\*)`).ReplaceAllString(content, "<i>$2</i>")
	// Combined bold and italic text
	content = regexp.MustCompile(`(\*\*\*)(.+?)(\*\*\*)`).ReplaceAllString(content, "<b><i>$2</i></b>")
	// Create codeblocks
	content = regexp.MustCompile("(`+)(.+?)(`+)").ReplaceAllString(content, "<code>$2</code>")

	return template.HTML(content)
}

func retrieveModmail(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(string(body), "# Modmail thread") {
		return "", fmt.Errorf("The provided URL is not a valid ModMail log thread.")
	}

	return string(body), nil
}

func tokeniseInfo(info []rune) (*ThreadInfo, error) {
	return nil, nil
}

func tokeniseThread(thread []rune, block int, buffer []rune, token *Token, tokens []*Token) ([]*Token, error) {
	if len(thread) == 0 {
		return tokens, nil
	}

	switch thread[0] {
	case '[':
		break
	case ']':
		if string(buffer) == "REPLY DELETED" {
			// TODO: Handle deleted replies
		}

		switch block {
		case 0:
			t, _ := time.Parse("2006-01-02 15:04:05", string(buffer))
			token.Time = t.Format(time.Stamp)
		case 1:
			token.Type = string(buffer)
		case 2:
			token.User = string(buffer)
		}

		buffer = []rune{}
		block++
	case '\n':
		if len(thread) > 1 && thread[1] != '[' {
			buffer = append(buffer, '\n')
			break
		}

		block = 0

		content := string(buffer)

		msg := Message{
			Content:  template.HTML(""),
			Edits:    []template.HTML{},
			Original: string(buffer[1:]),
		}

		if strings.Contains(content, "The user edited their message") {
			afterPos := strings.Index(content, "`A:` ")
			beforePos := strings.Index(content, "`B:` ")

			beforeContent := content[beforePos+len("`B:` ") : afterPos-1]
			for _, str := range regexp.MustCompile(`(?:www|https?)[^(\s|\n)]+`).FindAllString(beforeContent, -1) {
				beforeContent = strings.ReplaceAll(beforeContent, str, "<"+str+">")
			}

			for _, t := range tokens {
				for _, m := range t.Messages {
					if beforeContent == m.Original {
						m.Edits = append(m.Edits, template.HTML(content))
						//m.Content = processMessage(content[afterPos+len("`A:` "):])
					}
				}
			}

			break
		}

		if token.Type == "TO USER" {
			if strings.HasPrefix(content, " (Anonymous)") {
				split := strings.Split(content, ") ")[1]

				token.Anonymous = true
				token.Role = split[:strings.Index(split, ":")]
				content = split[strings.Index(split, ":")+2:]
			} else {
				role := regexp.MustCompile(`^ \(.*?\) `).FindString(content)
				token.Role = role[2 : len(role)-2]
				content = content[strings.Index(content, token.User+":")+len(token.User+": "):]
			}
		}

		tokenIndex := len(tokens) - 1
		if tokenIndex < 0 {
			tokenIndex = 0
		}

        msg.Content = processMessage(content)

		if len(tokens) > 0 && tokens[tokenIndex].User == token.User && tokens[tokenIndex].Type == token.Type {
			tokens[tokenIndex].Messages = append(tokens[len(tokens)-1].Messages, msg)
		} else {
			token.Messages = append(token.Messages, msg)
			tokens = append(tokens, token)
		}

		token = &Token{}
		buffer = []rune{}
	default:
		if len(thread) > 0 && thread[1] != '[' {
			buffer = append(buffer, rune(thread[0]))
		}
	}

	return tokeniseThread(thread[1:], block, buffer, token, tokens)
}

func parseModmail(thread string) (*ThreadInfo, []*Token, error) {
	split := strings.Split(thread, "\n────────────────\n")
	if len(split) < 2 {
		return nil, nil, fmt.Errorf("Modmail thread is not formatted correctly")
	}

	content := split[1] + "\n[2006-01-02 15:04:05] [END] End of ModMail\n\n"

	tokenisedInfo, err := tokeniseInfo([]rune(split[0]))
	if err != nil {
		return nil, nil, err
	}

	tokenisedThread, err := tokeniseThread([]rune(content), 0, []rune{}, &Token{}, []*Token{})
	if err != nil {
		return nil, nil, err
	}

	return tokenisedInfo, tokenisedThread, nil
}
