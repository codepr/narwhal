// BSD 2-Clause License
//
// Copyright (c) 2020, Andrea Giacomo Baldan
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package core

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

func handleDispatcherCommit(registry *RunnerRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// TODO
			// Wrap Commit struct inside a more generic Job structure,
			// tracking the state (e.g. PENDING, COMPLETE, FAILED) based
			// on runner's responses
			w.WriteHeader(http.StatusOK)
		case http.MethodPost:
			// Only POST is allowed, decode the json payload and check if the
			// received commit is elegible for a test-run of it's already been
			// processed before
			decoder := json.NewDecoder(r.Body)
			var c Commit
			err := decoder.Decode(&c)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				c.cTime = time.Now()
				if err := registry.EnqueueCommit(&c); err != nil {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}
		default:
			// 405 for unwanted HTTP methods
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func handleDispatcherRunner(registry *RunnerRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Return a list of already registered testrunners
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(registry.Runners())
		case http.MethodPost:
			// Register a new testrunner
			decoder := json.NewDecoder(r.Body)
			var s Runner = Runner{alive: true}
			err := decoder.Decode(&s)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			if err := registry.AddRunner(&s); err != nil {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		case http.MethodDelete:
			// Unregister testrunner
			decoder := json.NewDecoder(r.Body)
			var s Runner
			err := decoder.Decode(&s)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			registry.RemoveRunner(&s)
			w.WriteHeader(http.StatusNoContent)
		default:
			// 405 for unwanted HTTP methods
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func handleRunnerHealth(l *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}
}

func handleRunnerCommit(l *log.Logger) http.HandlerFunc {
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
			} else {
				c.cTime = time.Now()
				l.Printf("Running container for repository: %v\n", c.Repository.Name)
				errCh := RunContainer(&c)
				if err := <-errCh; err != nil {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}
		default:
			// 400 for unwanted HTTP methods
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}
