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
	"log"
	"testing"
	"time"
)

func newPool() *TestRunnerPool {
	ch := make(chan *Commit)
	return NewTestRunnerPool(ch, &log.Logger{})
}

func TestNewRunnerPool(t *testing.T) {
	pool := newPool()
	if pool == nil {
		t.Errorf("NewTestRunnerPool didn't create a valid object")
	}
	pool.Stop()
}

func TestRunnerPoolAddRunner(t *testing.T) {
	pool := newPool()
	if pool == nil {
		t.Errorf("NewTestRunnerPool didn't create a valid object")
	}
	testRunner := TestRunner{"http://localhost:8989", true}
	pool.AddRunner(testRunner)
	if len(pool.runners) == 0 {
		t.Errorf("TestRunnerPool.AddRunner didn't work, expected 1 got 0")
	}
	pool.Stop()
}

func TestPutCommit(t *testing.T) {
	pool := newPool()
	if pool == nil {
		t.Errorf("NewTestRunnerPool didn't create a valid object")
	}
	commit := Commit{"abcd123", "testrepo", time.Now().UTC()}
	pool.PutCommit("testrepo", &commit)
	if c, ok := pool.GetCommit("testrepo"); ok {
		if c != &commit {
			t.Errorf("TestRunnerPool.PutCommit didn't work")
		}
	}
	pool.Stop()
}
