package main

import (
	"flag"
	"github.com/codepr/dispatcher"
	"log"
	"os"
)

const healthcheck_timeout = 1

var addr string

func main() {
	flag.StringVar(&addr, "addr", ":28919", "Server listening address")
	flag.Parse()

	commitsCh := make(chan *dispacher.Commit)

	runnerPool := dispatcher.newTestRunnerPool(commitsCh)

	logger := log.New(os.Stdout, "dispatcher - ", log.LstdFlags)
	server := dispatcher.newServer(addr, logger, &runnerPool, healthcheck_timeout)

	log.Fatal(server.Run())
}
