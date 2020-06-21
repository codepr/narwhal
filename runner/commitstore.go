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

// Commitstore is the domain model of the dispatcher part of the application
// comprised of Commit, a simple abstraction over what we find useful to
// describe a commit and a CommitStore, which act as in-memory DB of the
// repositories tracked and their last processed commit

package runner

import (
	"fmt"
	"strings"
	"sync"
)

// Temporary database, should be replaced with a real DB, like sqlite
// Just carry a mapping of repository -> latest commit processed
type CommitStore struct {
	sync.Mutex
	repositories map[string]*CommitJob
}

type CommitJob struct {
	Id         string     `json:"id"`
	Language   string     `json:"language"`
	Repository Repository `json:"repository"`
	Specs      JobSpec    `json:"spec"`
}

type JobSpec struct {
	Dependencies []string `json:"dependencies"`
	Cmd          string   `json:"command"`
}

type CommitJobReply struct {
	Ok bool
}

func (c *CommitJob) Cmd() ([]string, error) {
	cloneCmd, err := c.Repository.CloneCommand("/" + c.Id)
	if err != nil {
		return nil, err
	}
	cmd := fmt.Sprintf("sh -c apt-get update && apt-get install -y %s && %s && %s",
		strings.Join(c.Specs.Dependencies, " "), cloneCmd, c.Specs.Cmd)
	return strings.Split(cmd, " "), nil
}

func (cs *CommitStore) PutCommit(c *CommitJob) {
	cs.Lock()
	cs.repositories[c.Repository.Name] = c
	cs.Unlock()
}

func (cs *CommitStore) GetCommit(repo string) (*CommitJob, bool) {
	cs.Lock()
	val, ok := cs.repositories[repo]
	cs.Unlock()
	return val, ok
}
