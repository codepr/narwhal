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

package backend

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

// CI configuration to be read from the file system on the cloned repository.
// For now it's queit simple:
// - A name
// - An image for the container to be used
// - Some environments variables
// - A list of steps to execute
//		- A name of the step
//		- Dependencies needed by the execution to be installed
//		- The command to execute
type CIConfig struct {
	Name      string            `yaml:"name"`
	ImageName string            `yaml:"image"`
	Env       map[string]string `yaml:"env,omitempty"`
	Steps     []struct {
		Name         string   `yaml:"name"`
		Dependencies []string `yaml:"dependencies,omitempty"`
		Cmd          string   `yaml:"command"`
	} `yaml:"steps"`
}

func loadFromFile(path string) (*CIConfig, error) {
	// XXX hardcoded
	// Set a default image `ubuntu`
	ciConfig := &CIConfig{ImageName: "ubuntu"}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, ciConfig)
	if err != nil {
		return nil, err
	}
	return ciConfig, nil
}
