package main

import (
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/heyLu/edn"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/heyLu/mu"
	"github.com/heyLu/mu/connection"
	"github.com/heyLu/mu/database"
	tx "github.com/heyLu/mu/transactor"
)

var dbUrl = "files://db?name=onelink"

func main() {
	isNew, err := mu.CreateDatabase(dbUrl)
	if err != nil {
		panic(err)
	}

	conn, err := mu.Connect(dbUrl)
	if err != nil {
		panic(err)
	}

	if isNew {
		mustTransactFile(conn, "schema.edn")
		mustTransactFile(conn, "init.edn")
	}

	router := mux.NewRouter()

	router.PathPrefix("/lib").Handler(http.StripPrefix("/lib", http.FileServer(http.Dir("lib"))))

	router.HandleFunc("/query", func(w http.ResponseWriter, req *http.Request) {
		q, err := edn.DecodeString(req.URL.Query().Get("q"))
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		db := conn.Db()
		res, err := mu.Q(q, db)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resEdn := new(bytes.Buffer)
		resEdn.WriteString("#{")
		first := true
		for tuple, _ := range res {
			if first {
				resEdn.WriteByte('\n')
				first = false
			}
			resEdn.WriteString("  ")
			resEdn.WriteByte('[')
			l := tuple.Length()
			for i := 0; i < l; i++ {
				resEdn.WriteString(fmt.Sprintf("%#v", tuple.ValueAt(i)))
				if i < l-1 {
					resEdn.WriteByte(' ')
				}
			}
			resEdn.WriteByte(']')
			resEdn.WriteByte('\n')
		}
		resEdn.WriteString("}")
		w.Write(resEdn.Bytes())
	})

	router.HandleFunc("/comment", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := req.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		decoder := schema.NewDecoder()
		var comment CommentForm
		err = decoder.Decode(&comment, req.PostForm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		db := conn.Db()
		res, err := mu.QString(`
{:find [?topic]
 :where [[?topic :topic/title _]]}`,
			db)
		if err != nil {
			log.Println(err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		var topicId int
		for k, _ := range res {
			topicId = k.ValueAt(0).(int)
		}

		txData := []tx.TxDatum{
			tx.Datum{
				Op: tx.Assert,
				E:  database.Id(topicId),
				A:  mu.Keyword("topic", "comments"),
				V:  tx.NewValue(mu.Tempid(mu.DbPartUser, -1))},
			tx.TxMap{
				Id: database.Id(mu.Tempid(mu.DbPartUser, -1)),
				Attributes: map[database.Keyword][]tx.Value{
					mu.Keyword("comment", "content"): []tx.Value{tx.NewValue(comment.Content)},
				},
			},
		}
		_, err = conn.Transact(txData)
		if err != nil {
			log.Println(err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, req, "/", http.StatusSeeOther)
	})

	router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		db := conn.Db()
		res, err := mu.QString(`
{:find [?topic ?title ?description ?url]
 :where [[?topic :topic/title ?title]
         [?topic :topic/description ?description]
         [?topic :topic/url ?url]]}`,
			db)
		if err != nil {
			log.Println(err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		for k, _ := range res {
			m := map[string]interface{}{
				"comments":    db.Entity(k.ValueAt(0).(int)).Get(mu.Keyword("topic", "comments")),
				"title":       k.ValueAt(1),
				"description": k.ValueAt(2),
				"url":         k.ValueAt(3),
			}
			err := indexTmpl.Execute(w, m)
			if err != nil {
				log.Println(err)
			}
		}
	})

	http.Handle("/", router)

	addr := "localhost:7777"
	log.Printf("Listening on http://%s\n", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		panic(err)
	}
}

type CommentForm struct {
	InReplyTo string `schema:"in-reply-to"`
	Content   string `schema:"content"`
	Author    string `schema:"author"`
}

type Comment struct {
	database.Entity
}

func NewComment(entity database.Entity) Comment {
	return Comment{entity}
}

func (c Comment) Id() string {
	return c.Get(mu.Keyword("comment", "id")).(string)
}

func (c Comment) Author() string {
	author := c.Get(mu.Keyword("comment", "author"))
	if author == nil {
		return "unknown"
	} else {
		return author.(database.Entity).Get(mu.Keyword("user", "name")).(string)
	}
}

func (c Comment) Content() string {
	return c.Get(mu.Keyword("comment", "content")).(string)
}

func (c Comment) Replies() []interface{} {
	return c.Get(mu.Keyword("comment", "replies")).([]interface{})
}

var sanitizePolicy *bluemonday.Policy

func init() {
	sanitizePolicy = bluemonday.StrictPolicy()
	sanitizePolicy.AllowElements("p", "em", "strong", "code", "pre", "a")
	sanitizePolicy.AllowStandardURLs()
	sanitizePolicy.AllowAttrs("href").OnElements("a")
	sanitizePolicy.AllowAttrs("class").OnElements("code")
	sanitizePolicy.AllowElements("ul", "ol", "li")
}

var tmplFuncs = template.FuncMap{
	"kw": mu.Keyword,
	"markdown": func(content string) template.HTML {
		htmlContent := blackfriday.MarkdownCommon([]byte(content))
		htmlContent = sanitizePolicy.SanitizeBytes(htmlContent)
		return template.HTML(htmlContent)
	},
	"comment": NewComment,
}

var indexTmpl = template.Must(template.New("index.html").
	Funcs(tmplFuncs).
	Parse(`
{{ define "Comment" }}
<article id="{{ .Id }}" class="comment" data-comment-id="{{ .Id }}">
  <span class="comment-meta">Written by {{ .Author }}</span>
  {{ .Content | markdown }}
  <section class="comments">
  {{ range $comment := .Replies }}
    {{ template "Comment" (comment $comment) }}
  {{ end }}
  </section>
</article>
{{ end }}

<!doctype html>
<html>
  <head>
    <title>{{ .title }} - onelink</title>
    <meta charset="utf-8" />
    <link rel="stylesheet" href="/lib/highlight.css" />
    <style>
    body {
      margin: 0;
    }

    #content {
      display: flex;
      align-items: center;
      justify-content: center;
      height: 100vh;
    }

    #topic {
      width: 80ex;
    }

    #topic h1 {
      text-align: center;
    }

    #topic > .comments {
      position: absolute;
      top: 100%;
      width: 80ex;
      padding-left: 0;
    }

    .comments { padding-left: 1.5em; }

    .comment-meta {
      color: #777;
    }

    .comment p:first-of-type {
      margin-top: 0.1ex;
    }

    .comment:target {
      background-color: rgba(245, 251, 0, 0.1);
    }

    .comment:target .comment {
      background-color: white;
    }

    #comment-form {
      margin-bottom: 3em;
    }

    #comment-form textarea {
      width: 100%;
    }

    #comment-form button[type="submit"] {
      float: right;
    }

    a {
      color: #555;
    }
    </style>
  </head>

  <body>
    <section id="content">
      <article id="topic">
        <h1><a href="{{ .url }}">{{ .title }}</a></h1>

        {{ markdown .description }}

        <section class="comments">
        <form id="comment-form" method="POST" action="/comment">
          <div class="field">
            <textarea name="content" required placeholder="Say something"></textarea>
          </div>
          <button type="submit">Post</button>
        </form>

        {{ range $comment := .comments }}
          {{ template "Comment" (comment $comment) }}
        {{ else }}
          <p>No comments yet</p>
        {{ end }}
        </section>
      </article>
    </section>

    <script src="/lib/highlight.js"></script>
    <script>hljs.initHighlightingOnLoad();</script>
  </body>
</html>
`))

func mustTransactFile(conn connection.Connection, file string) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}

	_, err = mu.TransactString(conn, string(data))
	if err != nil {
		panic(err)
	}

}
