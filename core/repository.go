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

package core

import (
	"errors"
	"fmt"
	"sync"
)

type HostingService string

const (
	GitHub    HostingService = "github"
	BitBucket HostingService = "bitbucket"
	GitLab                   = "gitlab"
)

type Repository struct {
	sync.Mutex
	HostingService HostingService `json:"hosting_service"`
	Name           string         `json:"name"`
	Branch         string         `json:"branch"`
	commitHistory  []*Commit
}

func (r *Repository) CloneCommand(path string) (string, error) {
	switch r.HostingService {
	case GitHub:
		return fmt.Sprintf("git clone -b %s https://github.com/%s %s",
			r.Branch, r.Name, path), nil
	case GitLab:
		return fmt.Sprintf("git clone -b %s https://gitlab.com/%s %s",
			r.Branch, r.Name, path), nil
	case BitBucket:
		return fmt.Sprintf("git clone -b %s https://bitbucket.com/%s %s",
			r.Branch, r.Name, path), nil
	}
	return "", errors.New(fmt.Sprintf("%s hosting service not supported",
		r.HostingService))
}

func (r *Repository) AddCommit(c *Commit) {
	r.Lock()
	r.commitHistory = append(r.commitHistory, c)
	r.Unlock()
}
