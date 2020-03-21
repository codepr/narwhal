package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const (
	roundRobin = iota
)

const (
	port                = ":28919"
	healthcheck_timeout = 1
	balancing           = roundRobin
)

var mux *http.ServeMux = http.NewServeMux()
var commitsCh chan *commit

// Just the URL of the testing machines for now
type testRunner struct {
	URL   string `json:"url"`
	Alive bool   `json:"alive"`
}

type commit struct {
	Id         string `json:"id"`
	Repository string `json:"repository"`
	cTime      time.Time
}

// Temporary database, should be replaced with a real DB, like sqlite
// Just carry a mapping of repository -> latest commit processed and an array
// of TestRunner servers
type store struct {
	runners      []testRunner
	repositories map[string]*commit
}

var repoStore = store{[]testRunner{}, map[string]*commit{}}

func (s *store) enqueueForTest(c *commit) {
	log.Println("Sending commit ", c)
	commitsCh <- c
}

func pushCommitToRunner(commitsCh chan *commit) {
	var counter int = 0
	var index int = 0
	for {
		select {
		case commit := <-commitsCh:
			runners := len(repoStore.runners)
			if runners == 0 {
				log.Println("No runners available")
				continue
			}
			if balancing == roundRobin {
				for index = counter % runners; repoStore.runners[index].Alive == false; {
					index = counter % runners
					counter++
				}
			}
			payload, err := json.Marshal(commit)
			if err != nil {
				log.Println("Unable to marshal", commit)
				continue
			}
			log.Println("Sending commit to %i runner", index)
			_, err = http.Post(repoStore.runners[index].URL+"/repository", "application/json", bytes.NewBuffer(payload))
			if err != nil {
				log.Println("Unable to send test to runner")
				continue
			}
		}
	}
}

// Simple middleware to log method and path
func reqLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path, 200)
		next(w, r)
	}
}

func handleCommit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Only POST is allowed, decode the json payload and check if the
		// received commit is elegible for a test-run of it's already been
		// processed before
		decoder := json.NewDecoder(r.Body)
		var c commit
		err := decoder.Decode(&c)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		c.cTime = time.Now()
		if _, ok := repoStore.repositories[c.Repository]; ok {
			if repoStore.repositories[c.Repository].Id != c.Id {
				log.Println("New commit received, enqueuing work to test runner")
				repoStore.repositories[c.Repository] = &c
				repoStore.enqueueForTest(&c)
			}
		} else {
			repoStore.repositories[c.Repository] = &c
			repoStore.enqueueForTest(&c)
		}
	default:
		// 400 for unwanted HTTP methods
		w.WriteHeader(http.StatusBadRequest)
	}
}

func handleTestRunner(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return a list of already registered testrunners
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repoStore.runners)
	case http.MethodPost:
		// Register a new testrunner
		decoder := json.NewDecoder(r.Body)
		var t testRunner
		err := decoder.Decode(&t)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		t.Alive = true
		repoStore.runners = append(repoStore.runners, t)
	default:
		// 400 for unwanted HTTP methods
		w.WriteHeader(http.StatusBadRequest)
	}
}

func main() {
	commitsCh = make(chan *commit)
	hc := make(chan bool)
	ticker := time.NewTicker(healthcheck_timeout * time.Second)
	// Run a goroutine to perform basic healthcheck on testrunners
	go func() {
		for {
			select {
			case <-hc:
				ticker.Stop()
				return
			case <-ticker.C:
				for _, t := range repoStore.runners {
					res, err := http.Get(t.URL + "/status")
					if err != nil || res.StatusCode != 200 {
						t.Alive = false
					}
				}
			}
		}
	}()
	go pushCommitToRunner(commitsCh)
	http.HandleFunc("/testrunner", reqLog(handleTestRunner))
	http.HandleFunc("/commit", reqLog(handleCommit))
	log.Println("Listening on", port)
	log.Fatal(http.ListenAndServe(port, nil))
	// Stop the healthcheck goroutine
	hc <- true
}
