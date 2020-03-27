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
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"
)

// Runner represents a worker unit on the network, it is identified by an URL,
// a commit-path (usually /commit) and an health-path for the healthcheck
// calls
type Runner struct {
	URL        string `json:"url"`
	CommitPath string `json:"commitpath"`
	HealthPath string `json:"healthpath"`
	alive      bool
}

// A central registry for all the registered runners, all runners operations
// should pass through this struct
type RunnerRegistry struct {
	// Lock to avoid contention
	m *sync.Mutex
	// A set of servers, each one consists of an URL and an Alive flag which
	// act as an indicator of reachability and thus availability for jobs
	// or containers to run tests in a safe and isolated environment
	// easier to reproduce the projects own test/production environments
	runners map[*Runner]bool
	// Current is the integer sentinel to be used to select an available
	// test-runner server to send job to using a round-robin algorithm
	current int
	// Store is just a pointer to a map of repositories -> commits. Each commit
	// value is updated at the last executed one
	store *CommitStore
	// Bounded channel of type *Commit, used as a simple task queue to notify
	// the job-dispatching goroutine to send work to selected test-runner
	commitQueue chan *Commit
	// Number of worker goroutine to start at init
	workers int
	// Just a logger to uniform with the rest of the app, generally it's the
	// server ErrorLog pointer
	logger *log.Logger
}

func (r *Runner) Forward(c *Commit) error {
	payload, err := json.Marshal(c)
	if err != nil {
		return errors.New("Unable to marshal commit")
	}
	url, _ := url.Parse(r.URL)
	url.Path = path.Join(url.Path, r.CommitPath)
	_, err = http.Post(url.String(), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return errors.New("Unable to send test to runner")
	}
	return nil
}

func (r *Runner) HealthCheck() {
	url, _ := url.Parse(r.URL)
	url.Path = path.Join(url.Path, r.HealthPath)
	res, err := http.Get(url.String())
	if err != nil || res.StatusCode != 200 {
		r.alive = false
	} else {
		r.alive = true
	}
}

func (r *Runner) Alive() bool {
	return r.alive
}

func NewRunnerRegistry(workers int, l *log.Logger) *RunnerRegistry {
	return &RunnerRegistry{
		m:       new(sync.Mutex),
		runners: map[*Runner]bool{},
		store: &CommitStore{
			repositories: map[string]*Commit{},
		},
		commitQueue: make(chan *Commit),
		workers:     workers,
		logger:      l,
	}
}

func (registry *RunnerRegistry) Start() {
	for i := 0; i < registry.workers; i++ {
		go registry.ForwardToRunner()
	}
}

func (registry *RunnerRegistry) Stop() {
	registry.logger.Println("Stopping registry")
	close(registry.commitQueue)
}

func (registry *RunnerRegistry) HealthCheck() {
	for t, _ := range registry.runners {
		t.HealthCheck()
	}
}

func (registry *RunnerRegistry) Runners() map[*Runner]bool {
	return registry.runners
}

func (registry *RunnerRegistry) AddRunner(r *Runner) error {
	registry.m.Lock()
	if _, ok := registry.runners[r]; ok {
		registry.m.Unlock()
		return errors.New("Runner already present in the registry")
	}
	registry.runners[r] = true
	registry.m.Unlock()
	return nil
}

func (registry *RunnerRegistry) RemoveRunner(r *Runner) {
	registry.m.Lock()
	delete(registry.runners, r)
	registry.m.Unlock()
}

func (registry *RunnerRegistry) ForwardToRunner() {
	for {
		commit, ok := <-registry.commitQueue
		// poison pill
		if ok == false {
			registry.logger.Println("Closing commit queue")
			return
		}
		// Obtain a valid ServerRunner instance, it must be alive, using round robin
		// to select it
		var index, i int = 0, 0
		registry.m.Lock()
		runners := len(registry.runners)
		if runners == 0 {
			registry.m.Unlock()
			registry.logger.Println("No runners available")
			continue
		}
		keys := make([]*Runner, runners)
		// Dumbest check to avoid looping forever in case of all dead servers
		abort := false
		for k := range registry.runners {
			if k.Alive() == true {
				abort = true
			}
			keys[i] = k
			i++
		}
		if abort == false {
			registry.logger.Println("No runners available")
			// TODO re-enqueue
			continue
		}
		// Round robin
		for index = registry.current % runners; keys[index].Alive() == false; {
			index = registry.current % runners
			registry.current++
		}
		registry.m.Unlock()

		err := keys[index].Forward(commit)
		if err != nil {
			registry.logger.Println(err)
		}
	}
}

func (registry *RunnerRegistry) EnqueueCommit(c *Commit) error {
	if cmt, ok := registry.store.GetCommit(c.Repository.Name); ok {
		if cmt.Id == c.Id {
			return errors.New("Commit already executed")
		}
	}
	registry.store.PutCommit(c)
	registry.commitQueue <- c
	return nil
}
