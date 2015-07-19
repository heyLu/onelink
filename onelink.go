package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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
	"github.com/heyLu/mu/index"
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

		txData := make([]tx.TxDatum, 0, 2)
		if comment.InReplyTo == "" { // (top-level) comment on a topic
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

			txData = append(txData, tx.Datum{
				Op: tx.Assert,
				E:  database.Id(topicId),
				A:  mu.Keyword("topic", "comments"),
				V:  tx.NewValue(mu.Tempid(mu.DbPartUser, -1))})
		} else { // reply to a comment
			txData = append(txData, tx.Datum{
				Op: tx.Assert,
				E: database.LookupRef{
					Attribute: mu.Keyword("comment", "id"),
					Value:     index.NewValue(comment.InReplyTo)},
				A: mu.Keyword("comment", "replies"),
				V: tx.NewValue(mu.Tempid(mu.DbPartUser, -1)),
			})
		}

		commentId := newRandomId()
		txData = append(txData,
			tx.TxMap{
				Id: database.Id(mu.Tempid(mu.DbPartUser, -1)),
				Attributes: map[database.Keyword][]tx.Value{
					mu.Keyword("comment", "id"):      []tx.Value{tx.NewValue(commentId)},
					mu.Keyword("comment", "content"): []tx.Value{tx.NewValue(comment.Content)},
				},
			})

		_, err = conn.Transact(txData)
		if err != nil {
			log.Println(err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, req, "/#"+commentId, http.StatusSeeOther)
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
  <span class="comment-meta">Written by {{ .Author }} (<a href="#{{ .Id }}">link</a>, <a href="#" class="comment-reply">reply</a>)</span>
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

    .comment {
      padding: 0.5ex;
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

    .comment-form {
      margin-bottom: 3em;
    }

    .comment-form textarea {
      width: 100%;
    }

    .comment-form .comment-controls {
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
        <form class="comment-form" method="POST" action="/comment">
          <div class="field">
            <textarea name="content" required placeholder="Say something"></textarea>
          </div>
          <div class="comment-controls">
            <button type="submit">Post</button>
          </div>
        </form>

        {{ range $comment := .comments }}
          {{ template "Comment" (comment $comment) }}
        {{ else }}
          <p>No comments yet</p>
        {{ end }}
        </section>
      </article>
    </section>

    <script>
    function handleReply(ev) {
      ev.preventDefault();
      var commentEl = ev.target.parentElement.parentElement;
      var commentId = commentEl.dataset.commentId;
      var replyId = "reply-" + commentId;

      var prevReplyEl = document.getElementById(replyId);
      if (prevReplyEl) {
        prevReplyEl.querySelector("textarea").focus();
        return;
      }

      var form = document.createElement("form");
      form.id = replyId;
      form.method = "POST";
      form.action = "/comment";
      form.className = "comment-form";

      var inReplyTo = document.createElement("input");
      inReplyTo.type = "hidden";
      inReplyTo.name = "in-reply-to";
      inReplyTo.value = commentId;
      form.appendChild(inReplyTo);

      var contentWrapper = document.createElement("div");
      contentWrapper.class = "field";
      form.appendChild(contentWrapper);

      var contentEl = document.createElement("textarea");
      contentEl.placeholder = "Say something";
      contentEl.required = "required";
      contentEl.name = "content";
      contentWrapper.appendChild(contentEl);

      var controls = document.createElement("div");
      controls.className = "comment-controls"
      form.appendChild(controls);

      var submitButton = document.createElement("button");
      submitButton.type = "submit";
      submitButton.textContent = "Post";
      controls.appendChild(submitButton);

      var cancelButton = document.createElement("button");
      cancelButton.textContent = "Cancel";
      cancelButton.addEventListener("click", function(ev) {
        ev.preventDefault();
        form.remove();
      });
      controls.appendChild(cancelButton);

      var comments = commentEl.querySelector(".comments");
      if (comments.firstChild == null) {
        comments.appendChild(form);
      } else {
        comments.insertBefore(form, comments.firstChild);
      }

      contentEl.focus();
    }

    var replyEls = document.querySelectorAll(".comment-reply")
    for (var i = 0; i < replyEls.length; i++) {
      replyEls[i].addEventListener("click", handleReply);
    }
    </script>
    <script src="/lib/highlight.js"></script>
    <script>hljs.initHighlightingOnLoad();</script>
  </body>
</html>
`))

func newRandomId() string {
	buf := make([]byte, 5)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

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
