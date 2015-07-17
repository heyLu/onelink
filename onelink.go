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
		res, err := mu.QString(`
{:find [?title ?description ?url]
 :where [[?topic :topic/title ?title]
         [?topic :topic/description ?description]
         [?topic :topic/url ?url]]}`,
			conn.Db())
		if err != nil {
			log.Println(err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		for k, _ := range res {
			log.Println(k)
			m := map[string]interface{}{
				"title":       k.ValueAt(0),
				"description": k.ValueAt(1),
				"url":         k.ValueAt(2),
			}
			indexTmpl.Execute(w, m)
		}
	})
	http.ListenAndServe("localhost:7777", nil)
}

var indexTmpl = template.Must(template.New("index.html").Parse(`<!doctype html>
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
      flex-direction: column;
      align-items: center;
      width: 100vw;
    }

    #topic {
      width: 80ex;
    }

    #topic h1 {
      text-align: center;
    }

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
