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
	"strings"
	"testing"
)

func TestPutCommit(t *testing.T) {
	store := CommitStore{repositories: map[string]*Commit{}}
	commit := Commit{Repository: Repository{Name: "test-repo"}}
	store.PutCommit(&commit)
	if len(store.repositories) == 0 {
		t.Errorf("PutCommit didn't add a commit before it existed in the store")
	}
	store.PutCommit(&commit)
	if len(store.repositories) == 0 || len(store.repositories) > 1 {
		t.Errorf("PutCommit didn't overwrite a commit that already existed in the store")
	}
	commitTwo := Commit{Repository: Repository{Name: "new-test-repo"}}
	store.PutCommit(&commitTwo)
	if len(store.repositories) < 2 {
		t.Errorf("PutCommit didn't add a commit before it existed in the store")
	}
}

func TestGetCommit(t *testing.T) {
	store := CommitStore{repositories: map[string]*Commit{}}
	commit := Commit{Repository: Repository{Name: "test-repo"}}
	store.PutCommit(&commit)
	if _, ok := store.GetCommit("test-repo"); ok == false {
		t.Errorf("GetCommit failed to fetch the commit")
	}
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestCmd(t *testing.T) {
	commit := Commit{
		Id: "ab23f",
		Repository: Repository{
			Name:           "johndoe/test-repo",
			HostingService: "github",
			Branch:         "master",
		},
	}
	cmd, err := commit.Cmd()
	expected := strings.Split("git clone -b master https://github.com/johndoe/test-repo /ab23f", " ")
	if err != nil {
		t.Errorf("Cmd errored: %s", err)
	} else {
		if equalStringSlices(cmd, expected) == false {
			t.Errorf("Cmd returned wrong clone string: %s", cmd)
		}
	}
}
