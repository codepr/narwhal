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

package dispatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

// Temporary database, should be replaced with a real DB, like sqlite
// Just carry a mapping of repository -> latest commit processed and an array
// of TestRunner servers
type Store struct {
	repositories map[string]*Commit
}

type Commit struct {
	Id         string `json:"id"`
	Repository string `json:"repository"`
	cTime      time.Time
}

// Just the URL of the testing machines for now
type TestRunner struct {
	URL   string `json:"url"`
	Alive bool   `json:"alive"`
}

type TestRunnerPool struct {
	runners   []TestRunner
	store     *Store
	commitsCh chan *Commit
}

func (tr *TestRunner) submitCommit(c *Commit) error {
	payload, err := json.Marshal(c)
	if err != nil {
		return errors.New("Unable to marshal commit")
	}
	_, err = http.Post(tr.URL+"/repository", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return errors.New("Unable to send test to runner")
	}
	return nil
}

func NewTestRunnerPool(ch chan *Commit) *TestRunnerPool {
	pool := TestRunnerPool{
		runners: []TestRunner{},
		store: &Store{
			repositories: map[string]*Commit{},
		},
		commitsCh: ch,
	}
	// Start goroutine to continually send commits incoming on the channel
	go pool.pushCommitToRunner()
	return &pool
}

func (pool *TestRunnerPool) AddRunner(t TestRunner) {
	pool.runners = append(pool.runners, t)
}

func (pool *TestRunnerPool) PutCommit(repo string, c *Commit) {
	pool.store.repositories[repo] = c
}

func (pool *TestRunnerPool) GetCommit(repo string) (*Commit, bool) {
	val, ok := pool.store.repositories[repo]
	return val, ok
}

// Obtain a valid TestRunner instance, it must be alive, using roudn robin to
// select it
func (pool *TestRunnerPool) getRunner() (*TestRunner, error) {
	var index, counter int = 0, 0
	runners := len(pool.runners)
	if runners == 0 {
		return nil, errors.New("No runners available")
	}
	// Round robin
	for index = counter % runners; pool.runners[index].Alive == false; {
		index = counter % runners
		counter++
	}
	return &pool.runners[index], nil
}

func (pool *TestRunnerPool) pushCommitToRunner() {
	for {
		select {
		case commit := <-pool.commitsCh:
			runner, err := pool.getRunner()
			if err != nil {
				log.Println(err)
				continue
			}
			log.Println("Sending commit to runner")
			runner.submitCommit(commit)
		}
	}
}

func (pool *TestRunnerPool) EnqueueCommit(c *Commit) {
	pool.commitsCh <- c
}
