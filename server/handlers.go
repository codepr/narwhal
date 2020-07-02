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

package server

import (
	"encoding/json"
	"github.com/codepr/narwhal/runner"
	"net/http"
)

func handleDispatcherCommit(registry *runner.RunnerRegistry) http.HandlerFunc {
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
			var c runner.CommitJob
			err := decoder.Decode(&c)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			} else {
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

func handleDispatcherRunner(registry *runner.RunnerRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Return a list of already registered testrunners
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(registry.RunnerProxys())
		case http.MethodPost:
			// Register a new testrunner
			decoder := json.NewDecoder(r.Body)
			var s runner.RunnerProxy = runner.RunnerProxy{}
			err := decoder.Decode(&s)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			if err := registry.AddRunnerProxy(&s); err != nil {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		case http.MethodDelete:
			// Unregister testrunner
			decoder := json.NewDecoder(r.Body)
			var s runner.RunnerProxy
			err := decoder.Decode(&s)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			registry.RemoveRunnerProxy(&s)
			w.WriteHeader(http.StatusNoContent)
		default:
			// 405 for unwanted HTTP methods
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}
