package dispatcher

import (
	"encoding/json"
	"net/http"
	"time"
)

func handleCommit(pool *TestRunnerPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Only POST is allowed, decode the json payload and check if the
			// received commit is elegible for a test-run of it's already been
			// processed before
			decoder := json.NewDecoder(r.Body)
			var c Commit
			err := decoder.Decode(&c)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			c.cTime = time.Now()
			if cmt, ok := pool.GetCommit(c.Repository); ok {
				if cmt.Id != c.Id {
					pool.PutCommit(c.Repository, &c)
					pool.EnqueueCommit(&c)
				}
			} else {
				pool.PutCommit(c.Repository, &c)
				pool.EnqueueCommit(&c)
			}
			w.WriteHeader(http.StatusOK)
		default:
			// 400 for unwanted HTTP methods
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

func handleTestRunner(pool *TestRunnerPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Return a list of already registered testrunners
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(pool.runners)
		case http.MethodPost:
			// Register a new testrunner
			decoder := json.NewDecoder(r.Body)
			var t TestRunner
			err := decoder.Decode(&t)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			t.Alive = true
			pool.AddRunner(t)
			w.WriteHeader(http.StatusOK)
		default:
			// 400 for unwanted HTTP methods
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}
