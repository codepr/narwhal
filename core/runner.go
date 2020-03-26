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
)

type Runner struct {
	URL   string `json:"url"`
	alive bool
}

func (r *Runner) Forward(c *Commit) error {
	payload, err := json.Marshal(c)
	if err != nil {
		return errors.New("Unable to marshal commit")
	}
	url, _ := url.Parse(r.URL)
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

func (tr *Runner) HealthCheck() {
	res, err := http.Get(tr.URL + "/health")
	if err != nil || res.StatusCode != 200 {
		tr.alive = false
	} else {
		tr.alive = true
	}
}

func (r *Runner) Alive() bool {
	return r.alive
}
