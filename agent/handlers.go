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

package agent

import (
	. "github.com/codepr/narwhal/backend"
	"github.com/google/go-github/v32/github"
	"log"
	"net/http"
)

func healthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func commitHandler(events chan<- Commit) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := github.ValidatePayload(r, []byte("my-secret-key"))
		if err != nil {
			log.Printf("error validating request body: err=%s\n", err)
			return
		}
		defer r.Body.Close()

		event, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			log.Printf("could not parse webhook: err=%s\n", err)
			return
		}

		switch e := event.(type) {
		case *github.PushEvent:
			// Push it into events channel
			headCommit := e.GetHeadCommit()
			repo := e.GetRepo()
			id, timestamp := headCommit.GetID(), headCommit.Timestamp
			lang, name, branch := repo.Language, repo.FullName, repo.DefaultBranch
			commit := Commit{
				Id:        id,
				Timestamp: timestamp.Time,
				Language:  *lang,
				Repository: Repository{
					HostingService: GitHub,
					Name:           *name,
					Branch:         *branch,
				},
			}
			events <- commit
		default:
			log.Printf("Ignored event type %s\n", github.WebHookType(r))
			return
		}
	}
}
