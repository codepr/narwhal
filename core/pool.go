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
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"log"
	"sync"
)

const (
	registry string = "docker.io/library/"
	image    string = "ubuntu"
)

type RunnerPool struct {
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

type DockerPool struct {
	ctx    context.Context
	c      *client.Client
	logger *log.Logger
}

func NewRunnerPool(workers int, l *log.Logger) *RunnerPool {
	pool := RunnerPool{
		m:       new(sync.Mutex),
		runners: map[*Runner]bool{},
		store: &CommitStore{
			repositories: map[string]*Commit{},
		},
		commitQueue: make(chan *Commit),
		workers:     workers,
		logger:      l,
	}
	return &pool
}

func (pool *RunnerPool) Start() {
	for i := 0; i < pool.workers; i++ {
		go pool.ForwardToRunner()
	}
}

func (pool *RunnerPool) Stop() {
	pool.logger.Println("Stopping pool")
	close(pool.commitQueue)
}

func (pool *RunnerPool) HealthCheck() {
	for t, _ := range pool.runners {
		t.HealthCheck()
	}
}

func (pool *RunnerPool) Runners() map[*Runner]bool {
	return pool.runners
}

func (pool *RunnerPool) AddRunner(r *Runner) error {
	pool.m.Lock()
	if _, ok := pool.runners[r]; ok {
		pool.m.Unlock()
		return errors.New("Runner already present in the pool")
	}
	pool.runners[r] = true
	pool.m.Unlock()
	return nil
}

func (pool *RunnerPool) RemoveRunner(r *Runner) {
	pool.m.Lock()
	delete(pool.runners, r)
	pool.m.Unlock()
}

func (pool *RunnerPool) ForwardToRunner() {
	for {
		commit, ok := <-pool.commitQueue
		// poison pill
		if ok == false {
			pool.logger.Println("Closing commit queue")
			return
		}
		// Obtain a valid ServerRunner instance, it must be alive, using round robin
		// to select it
		var index, i int = 0, 0
		pool.m.Lock()
		runners := len(pool.runners)
		if runners == 0 {
			pool.m.Unlock()
			pool.logger.Println("No runners available")
			continue
		}
		keys := make([]*Runner, runners)
		// Dumbest check to avoid looping forever in case of all dead servers
		abort := false
		for k := range pool.runners {
			if k.Alive() == true {
				abort = true
			}
			keys[i] = k
			i++
		}
		if abort == false {
			pool.logger.Println("No runners available")
			// TODO re-enqueue
			continue
		}
		// Round robin
		for index = pool.current % runners; keys[index].Alive() == false; {
			index = pool.current % runners
			pool.current++
		}
		pool.m.Unlock()

		err := keys[index].Forward(commit)
		if err != nil {
			pool.logger.Println(err)
		}
	}
}

func (pool *RunnerPool) EnqueueCommit(c *Commit) error {
	if cmt, ok := pool.store.GetCommit(c.Repository.Name); ok {
		if cmt.Id == c.Id {
			return errors.New("Commit already executed")
		}
	}
	pool.store.PutCommit(c)
	pool.commitQueue <- c
	return nil
}

// ContainerRunnerPool builder func
func NewDockerPool(l *log.Logger) (*DockerPool, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	pool := DockerPool{
		logger: l,
		ctx:    context.Background(),
		c:      cli,
	}
	return &pool, nil
}

func (pool *DockerPool) RunContainer(c *Commit) error {
	_, err := pool.c.ImagePull(pool.ctx, registry+image,
		types.ImagePullOptions{})
	if err != nil {
		return err
	}
	cmd, err := c.CloneRepositoryCmd("/" + c.Id)
	if err != nil {
		return err
	}
	resp, err := pool.c.ContainerCreate(pool.ctx, &container.Config{
		Image: image,
		Cmd:   cmd,
	}, nil, nil, "")
	if err != nil {
		return err
	}

	if err := pool.c.ContainerStart(pool.ctx, resp.ID,
		types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

func (pool *DockerPool) EnqueueCommit(c *Commit) error {
	go pool.RunContainer(c)
	return nil
}
