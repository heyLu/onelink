package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/heyLu/mu"
	"github.com/heyLu/mu/connection"
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

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
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
	http.ListenAndServe("localhost:7777", nil)
}

var tmplFuncs = template.FuncMap{
	"kw": mu.Keyword,
}

var indexTmpl = template.Must(template.New("index.html").
	Funcs(tmplFuncs).
	Parse(`
{{ define "Comment" }}
<article class="comment">
  <p>{{ .Get (kw "comment" "content") }}</p>
  <section class="comments">
  {{ range $comment := .Get (kw "comment" "replies") }}
    {{ template "Comment" $comment }}
  {{ end }}
  </section>
</article>
{{ end }}

<!doctype html>
<html>
  <head>
    <title>{{ .title }} - onelink</title>
    <meta charset="utf-8" />
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
      padding-left: 0;
    }

    .comments { padding-left: 1.5em; }

    a {
      color: #555
    }
    </style>
  </head>

  <body>
    <section id="content">
      <article id="topic">
        <h1><a href="{{ .url }}">{{ .title }}</a></h1>

        <p>{{ .description }}
        </p>

        <section class="comments">
        {{ range $comment := .comments }}
          {{ template "Comment" $comment }}
        {{ else }}
          <p>No comments yet</p>
        {{ end }}
        </section>
      </article>
    </section>
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
