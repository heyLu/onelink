package main

import (
	"io/ioutil"

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
