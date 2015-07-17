package main

import (
	"io/ioutil"

	"github.com/heyLu/mu"
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
		data, err := ioutil.ReadFile("schema.edn")
		if err != nil {
			panic(err)
		}

		_, err = mu.TransactString(conn, string(data))
		if err != nil {
			panic(err)
		}
	}
}
