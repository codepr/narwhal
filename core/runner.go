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
	"context"
	"encoding/json"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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

const registryPrefix = "docker.io/library/"

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
	// Start the runner pool
	Start()

	// Stop the runner pool
	Stop()

	// Enqueue a new commit to be executed
	EnqueueCommitExecution(*Commit) error
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
	containerID    string
	containerImage string
	state          ContainerState
	busy           bool
	jobsExecuted   int
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
	runners map[*ServerRunner]bool

	// Current is the integer sentinel to be used to select an available
	// test-runner server to send job to using a round-robin algorithm
	current int

	// Store is just a pointer to a map of repositories -> commits. Each commit
	// value is updated at the last executed one
	store *CommitStore

	// Bounded channel of type *Commit, used as a simple task queue to notify
	// the job-dispatching goroutine to send work to selected test-runner
	commitQueue chan *CommitJob

	// Just a logger to uniform with the rest of the app, generally it's the
	// server ErrorLog pointer
	logger *log.Logger
}

// Pool of containers
type ContainerRunnerPool struct {
	// Lock to avoid contention
	m *sync.Mutex

	// Array of containers
	containers []*ContainerRunner

	// Current is the integer sentinel to be used to select an available
	// test-runner server to send job to using a round-robin algorithm
	current int

	// Backgorund context to establish docker container creation deadline
	ctx context.Context

	// Docker client to manage containers on the host machine
	c *client.Client

	// Bounded channel of type *Commit, used as a simple task queue to notify
	// the job-dispatching goroutine to send work to selected test-runner
	commitQueue chan *CommitJob

	// Just a logger to uniform with the rest of the app, generally it's the
	// server ErrorLog pointer
	logger *log.Logger
}

// Just a TestRunnerPool builder function
func NewTestRunnerPool(ch chan *CommitJob, l *log.Logger) *TestRunnerPool {
	pool := TestRunnerPool{
		m:       new(sync.Mutex),
		runners: map[*ServerRunner]bool{},
		store: &CommitStore{
			repositories: map[string]*Commit{},
		},
		commitQueue: ch,
		logger:      l,
	}
	return &pool
}

// ContainerRunnerPool builder func
func NewContainerRunnerPool(ch chan *CommitJob, l *log.Logger,
	images []string) (*ContainerRunnerPool, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	pool := ContainerRunnerPool{
		m:           new(sync.Mutex),
		containers:  []*ContainerRunner{},
		commitQueue: ch,
		logger:      l,
		ctx:         context.Background(),
		c:           cli,
	}
	for _, img := range images {
		container := ContainerRunner{
			containerImage: img,
			state:          INITING,
			busy:           false,
			jobsExecuted:   0,
		}
		pool.containers = append(pool.containers, &container)
	}
	return &pool, nil
}

// ServerRunner interface function imlpementations

// Submit function to submit a commit to the URL associated to the
// ServerRunner object
func (tr ServerRunner) Submit(c *Commit) error {
	payload, err := json.Marshal(c)
	if err != nil {
		return errors.New("Unable to marshal commit")
	}
	url, _ := url.Parse(tr.URL)
	url.Path = path.Join(url.Path, "/commit")
	log.Println("Sending req to", url.String())
	res, err := http.Post(url.String(), "application/json", bytes.NewBuffer(payload))
	log.Println("Sent req to", url.String())
	if err != nil {
		return errors.New("Unable to send test to runner")
	}
	log.Println(res.StatusCode)
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
	} else {
		tr.alive = true
	}
}

// ContainerRunner interface functions implementations

func (cr ContainerRunner) Submit(c *Commit) error {
	// TODO
	// ideally we want to run the code inside the container in a pure ephemeral
	// without mapping volumes on the disk and storing any sort of state
	log.Println("Received job")
	return nil
}

func (cr ContainerRunner) Alive() bool {
	return true
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

func (pool *TestRunnerPool) Runners() map[*ServerRunner]bool {
	return pool.runners
}

func (pool *TestRunnerPool) AddRunner(r *ServerRunner) error {
	pool.m.Lock()
	if _, ok := pool.runners[r]; ok {
		pool.m.Unlock()
		return errors.New("Runner already present in the pool")
	}
	pool.runners[r] = true
	pool.m.Unlock()
	return nil
}

func (pool *TestRunnerPool) RemoveRunner(r *ServerRunner) {
	pool.m.Lock()
	delete(pool.runners, r)
	pool.m.Unlock()
}

func (pool TestRunnerPool) EnqueueCommitExecution(c *Commit) error {
	if cmt, ok := pool.store.GetCommit(c.Repository); ok {
		if cmt.Id == c.Id {
			return errors.New("Commit already executed")
		}
	}
	pool.store.PutCommit(c)
	// Obtain a valid ServerRunner instance, it must be alive, using round robin
	// to select it
	var index, i int = 0, 0
	pool.m.Lock()
	runners := len(pool.runners)
	if runners == 0 {
		pool.m.Unlock()
		return errors.New("No runners available")
	}
	keys := make([]*ServerRunner, runners)
	for k := range pool.runners {
		pool.logger.Println(k.URL, k.alive, k.Alive())
		keys[i] = k
		i++
	}
	// Round robin
	for index = pool.current % runners; keys[index].Alive() == false; {
		index = pool.current % runners
		pool.current++
	}
	pool.m.Unlock()

	pool.commitQueue <- &CommitJob{c, keys[index]}

	return nil
}

func (pool TestRunnerPool) Start() {
	// Start goroutine to continually send commits incoming on the channel
	// anonymous function meant to be executed concurrently as a goroutine,
	// continously polling for new commits to forward them to an alive runner
	go func() {
		for {
			commitJob, ok := <-pool.commitQueue
			// poison pill
			if ok == false {
				pool.logger.Println("Closing commit queue")
				return
			}
			pool.logger.Printf("Sending commit %s to runner\n",
				commitJob.commit.Id)
			err := commitJob.runner.Submit(commitJob.commit)
			if err != nil {
				pool.logger.Println(err)
			}
		}
	}()
}

func (pool TestRunnerPool) Stop() {
	pool.logger.Println("Stopping pool")
	close(pool.commitQueue)
}

func (pool *TestRunnerPool) HealthCheck() {
	for t, _ := range pool.runners {
		t.HealthCheck()
	}
}

// ContainerRunnerPool funcs

func (pool ContainerRunnerPool) Start() {
	// Start goroutine to continually send commits incoming on the channel
	// anonymous function meant to be executed concurrently as a goroutine,
	// continously polling for new commits to forward them to an alive runner
	go func() {
		for {
			commitJob, ok := <-pool.commitQueue
			if ok == false {
				pool.logger.Println("Closing commit queue")
				return
			}
			pool.logger.Printf("Sending commit %s to container\n",
				commitJob.commit.Id)
			err := commitJob.runner.Submit(commitJob.commit)
			if err != nil {
				pool.logger.Println(err)
			}
		}
	}()
	pool.m.Lock()
	for _, container := range pool.containers {
		pool.initRunner(container)
	}
	pool.m.Unlock()
}

func (pool ContainerRunnerPool) Stop() {
	close(pool.commitQueue)
}

func (pool ContainerRunnerPool) EnqueueCommitExecution(c *Commit) error {
	// Obtain a valid ServerRunner instance, it must be alive, using round robin
	// to select it
	var index int = 0
	pool.m.Lock()
	containers := len(pool.containers)
	if containers == 0 {
		pool.m.Unlock()
		return errors.New("No containers available")
	}
	// Round robin
	for index = pool.current % containers; pool.containers[index].Alive() == false; {
		index = pool.current % containers
		pool.current++
	}
	pool.m.Unlock()

	pool.commitQueue <- &CommitJob{c, pool.containers[index]}
	log.Printf("Commit sent to %s container\n", pool.containers[index].containerID)

	return nil
}

func (pool *ContainerRunnerPool) initRunner(cr *ContainerRunner) error {
	reader, err := pool.c.ImagePull(pool.ctx, registryPrefix+cr.containerImage,
		types.ImagePullOptions{})
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, reader)
	resp, err := pool.c.ContainerCreate(pool.ctx, &container.Config{
		Image: cr.containerImage,
	}, nil, nil, "")
	if err != nil {
		return err
	}

	if err := pool.c.ContainerStart(pool.ctx, resp.ID,
		types.ContainerStartOptions{}); err != nil {
		return err
	}

	cr.state = RUNNING

	return nil
}
