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
	"sync"
)

type ContainerState int

const (
	INITING = iota
	RUNNING
	STOPPED
	CRASHED
	RESTARTING
)

// Runner defines the behaviour of a generic test runner, be it a server entity
// or a container for test execution
type Runner interface {
	// Submit job
	// TODO return a JobResult or something
	Submit(*Commit) error

	// Describe the status of the runner
	Alive() bool

	// Set alive flag
	SetAlive(bool)

	// Check for alive status of the runner
	HealthCheck()
}

type RunnerPool interface {
	// Enqueue commit execution
	EnqueueCommitExecution(*Commit)

	// Return the runners registered on the pool
	Runners() map[Runner]bool

	// Check for health of every runner
	HealthCheck()

	// Stop the runner
	Stop()

	// Add a new runner to the pool
	AddRunner(Runner)

	// Remove an existing runner, returning a boolean true if all went ok,
	// false if no runner was found
	RemoveRunner(Runner)

	// Update the latest processed commit on the store map
	PutCommit(string, *Commit)

	// Retrieve the latest commit from the store
	GetCommit(string) (*Commit, bool)
}

// Just the URL of the testing machines for now
type ServerRunner struct {
	URL   string `json:"url"`
	alive bool
}

// A generic wrapper around a docker container, it carries the identifier of
// the container and some stats/flags to describe its general status and
// availability over time
type ContainerRunner struct {
	containerID  string
	state        ContainerState
	busy         bool
	jobsExecuted int
}

// Pool of servers to be targeted for incoming jobs (e.g. new commits to run
// tests against)
type TestRunnerPool struct {
	// Lock to avoid contention
	m *sync.Mutex

	// A set of servers, each one consists of an URL and an Alive flag which
	// act as an indicator of reachability and thus availability for jobs
	// or containers to run tests in a safe and isolated environment
	// easier to reproduce the projects own test/production environments
	runners map[Runner]bool

	// Current is the integer sentinel to be used to select an available
	// test-runner server to send job to using a round-robin algorithm
	current int

	// Store is just a pointer to a map of repositories -> commits. Each commit
	// value is updated at the last executed one
	store *CommitStore

	// Bounded channel of type *Commit, used as a simple task queue to notify
	// the job-dispatching goroutine to send work to selected test-runner
	commitsCh chan *Commit

	// Just a logger to uniform with the rest of the app, generally it's the
	// server ErrorLog pointer
	logger *log.Logger
}

// Just a TestRunnerPool builder function
func NewTestRunnerPool(ch chan *Commit, l *log.Logger) *TestRunnerPool {
	pool := TestRunnerPool{
		m:       new(sync.Mutex),
		runners: map[Runner]bool{},
		store: &CommitStore{
			repositories: map[string]*Commit{},
		},
		commitsCh: ch,
		logger:    l,
	}
	// Start goroutine to continually send commits incoming on the channel
	go pool.pushCommitToRunner()
	return &pool
}

// ServerRunner interface function imlpementations

// Submit function to submit a commit to the URL associated to the
// ServerRunner object
func (tr ServerRunner) Submit(c *Commit) error {
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

func (tr ServerRunner) Alive() bool {
	return tr.alive
}

func (tr ServerRunner) SetAlive(alive bool) {
	tr.alive = alive
}

func (tr ServerRunner) HealthCheck() {
	res, err := http.Get(tr.URL + "/health")
	if err != nil || res.StatusCode != 200 {
		tr.SetAlive(false)
	}
	log.Println(res.StatusCode)
}

// ContainerRunner interface functions implementations

func (cr ContainerRunner) Submit(c *Commit) error {
	// TODO
	// ideally we want to run the code inside the container in a pure ephemeral
	// without mapping volumes on the disk and storing any sort of state
	return nil
}

func (cr ContainerRunner) Alive() bool {
	return cr.state == RUNNING
}

func (cr ContainerRunner) SetAlive(alive bool) {
	if alive == true {
		cr.state = RUNNING
	} else {
		cr.state = STOPPED
	}
}

func (cr ContainerRunner) HealthCheck() {
	// TODO
	// Check for docker client APIs
}

// TestRunnerPool interface function implementations

func (pool *TestRunnerPool) Runners() map[Runner]bool {
	return pool.runners
}

func (pool *TestRunnerPool) AddRunner(r Runner) {
	pool.m.Lock()
	pool.runners[r] = true
	pool.m.Unlock()
}

func (pool *TestRunnerPool) RemoveRunner(r Runner) {
	pool.m.Lock()
	delete(pool.runners, r)
	pool.m.Unlock()
}

func (pool *TestRunnerPool) PutCommit(repo string, c *Commit) {
	pool.store.PutCommit(repo, c)
}

func (pool *TestRunnerPool) GetCommit(repo string) (*Commit, bool) {
	return pool.store.GetCommit(repo)
}

// Obtain a valid ServerRunner instance, it must be alive, using round robin
// to select it
func (pool *TestRunnerPool) getRunner() (Runner, error) {
	var index, i int = 0, 0
	pool.m.Lock()
	runners := len(pool.runners)
	if runners == 0 {
		pool.m.Unlock()
		return nil, errors.New("No runners available")
	}
	keys := make([]Runner, runners)
	for k := range pool.runners {
		keys[i] = k
		i++
	}
	// Round robin
	for index = pool.current % runners; keys[index].Alive() == false; {
		index = pool.current % runners
		pool.current++
	}
	pool.m.Unlock()
	return keys[index], nil
}

// Private function meant to be executed concurrently as a goroutine,
// continously polling for new commits to forward them to an alive runner
func (pool *TestRunnerPool) pushCommitToRunner() {
	for {
		select {
		case commit := <-pool.commitsCh:
			runner, err := pool.getRunner()
			if err != nil {
				pool.logger.Println(err)
				continue
			}
			pool.logger.Println("Sending commit to runner")
			err = runner.Submit(commit)
			if err != nil {
				pool.logger.Println(err)
			}
		case <-pool.commitsCh:
			return
		}
	}
}

func (pool TestRunnerPool) EnqueueCommitExecution(c *Commit) {
	pool.commitsCh <- c
}

func (pool TestRunnerPool) Stop() {
	close(pool.commitsCh)
}

func (pool TestRunnerPool) HealthCheck() {
	for t, _ := range pool.runners {
		t.HealthCheck()
	}
}