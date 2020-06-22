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

package runner

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"log"
	"net/rpc"
	"sync"
)

const (
	registry string = "docker.io/library/"
	image    string = "ubuntu"
)

// Runner represents a worker unit on the network, it is identified by an URL,
// a commit-path (usually /commit) and an health-path for the healthcheck
// calls
type Runner struct {
	Addr      string `json:"addr"`
	rpcClient *rpc.Client
}

// A central registry for all the registered runners, all runners operations
// should pass through this struct
type RunnerRegistry struct {
	// Lock to avoid contention
	sync.Mutex
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
	// Just a logger to uniform with the rest of the app, generally it's the
	// server ErrorLog pointer
	logger *log.Logger
}

func (r *Runner) ExecuteCommitJob(c CommitJob, jr *CommitJobReply) error {
	go func() {
		ctx := context.Background()
		cli, err := client.NewEnvClient()
		if err != nil {
			jr.Ok = false
			return
		}
		log.Println("Executing commit job")
		log.Printf("Creating container %s\n", registry+image)
		// TODO stub
		_, err = cli.ImagePull(ctx, registry+image, types.ImagePullOptions{})
		if err != nil {
			jr.Ok = false
			return
		}
		cmd, err := c.Cmd()
		if err != nil {
			jr.Ok = false
			return
		}
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image: image,
			Cmd:   cmd,
		}, nil, nil, "")
		if err != nil {
			jr.Ok = false
			return
		}

		if err := cli.ContainerStart(ctx, resp.ID,
			types.ContainerStartOptions{}); err != nil {
			return
		}
		jr.Ok = true
		return
	}()
	return nil
}

func NewRunnerRegistry(l *log.Logger) *RunnerRegistry {
	return &RunnerRegistry{
		runners: map[*Runner]bool{},
		store: &CommitStore{
			repositories: map[string]*CommitJob{},
		},
		logger: l,
	}
}

func (registry *RunnerRegistry) Runners() map[*Runner]bool {
	return registry.runners
}

func (registry *RunnerRegistry) AddRunner(r *Runner) error {
	registry.Lock()
	defer registry.Unlock()
	if _, ok := registry.runners[r]; ok {
		return errors.New("Runner already present in the registry")
	}
	client, err := rpc.Dial("tcp", r.Addr)
	if err != nil {
		return err
	}
	r.rpcClient = client
	registry.runners[r] = true
	return nil
}

func (registry *RunnerRegistry) RemoveRunner(r *Runner) {
	registry.Lock()
	r.rpcClient.Close()
	delete(registry.runners, r)
	registry.Unlock()
}

func (registry *RunnerRegistry) forwardToRunner(c *CommitJob) {
	// Obtain a valid ServerRunner instance, it must be alive, using round robin
	// to select it
	var index, i int = 0, 0
	registry.Lock()
	runners := len(registry.runners)
	if runners == 0 {
		registry.Unlock()
		registry.logger.Println("No runners available")
		return
	}
	keys := make([]*Runner, runners)
	// Dumbest check to avoid looping forever in case of all dead servers
	for k := range registry.runners {
		keys[i] = k
		i++
	}
	// Round robin
	index = registry.current % runners
	registry.current++
	registry.Unlock()

	var jobReply CommitJobReply
	err := keys[index].rpcClient.Call("Runner.ExecuteCommitJob", c, &jobReply)
	if err != nil {
		log.Println("Unable to send test to runner")
	}
	if err != nil {
		registry.logger.Println(err)
	}
}

func (registry *RunnerRegistry) EnqueueCommit(c *CommitJob) error {
	if cmt, ok := registry.store.GetCommit(c.Repository.Name); ok {
		if cmt.Id == c.Id {
			return errors.New("Commit already executed")
		}
	}
	registry.store.PutCommit(c)
	go registry.forwardToRunner(c)
	return nil
}
