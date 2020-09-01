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
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-git/go-git/v5"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"path"
	"strings"
	"time"
)

const TEMPDIR string = "/tmp/"

type RunnerRequest struct {
	CommitJob Commit
}

type RunnerResponse struct {
	Response string
}

type HeartBeatRequest struct{}

type HeartBeatResponse struct {
	Alive bool
}

type Runner struct{}

func (r *Runner) HeartBeat(req HeartBeatRequest, res *HeartBeatResponse) error {
	res.Alive = true
	return nil
}

func cloneRepository(name string) (string, error) {
	// Tempdir to clone the repository
	dir, err := ioutil.TempDir(TEMPDIR, name)
	if err != nil {
		return "", err
	}

	// Clones the repository into the given dir, just as a normal git clone does
	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL: path.Join("https://github.com", name),
	})

	if err != nil {
		return "", err
	}

	return dir, nil
}

func createDockerfile(dir, imageName, cmd string, deps []string) error {
	f, err := os.Create(path.Join(dir, "Dockerfile"))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	dockerfile := fmt.Sprintf("FROM %s\nCOPY . /build\nRUN %s\nCMD %s",
		imageName, cmd, strings.Join(deps[:], ","))
	_, err = w.WriteString(dockerfile)
	if err != nil {
		return err
	}
	w.Flush()
	return nil
}

func runContainer(ciConfig *CIConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, err := docker.NewEnvClient()
	if err != nil {
		panic(err)
	}

	reader, err := cli.ImagePull(ctx, "docker.io/library/alpine",
		types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: ciConfig.ImageName,
		Cmd:   []string{"echo", "hello world"},
		Tty:   false,
	}, nil, nil, "")
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	_, err = cli.ContainerWait(ctx, resp.ID)
	if err != nil {
		panic(err)
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		panic(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}

func (r *Runner) RunCommitJob(req RunnerRequest, res *RunnerResponse) error {
	dir, err := cloneRepository(req.CommitJob.GetRepositoryName())
	if err != nil {
		return err
	}
	// Delete temporary at the end of the execution
	defer os.RemoveAll(dir)

	// Read CI configuration
	ciConfig, err := LoadCIConfigFromFile(dir)
	if err != nil {
		res.Response = "NOK"
		return err
	}
	// Create a Dockerfile in the tempdir
	createDockerfile(dir, ciConfig.ImageName, ciConfig.Steps[0].Cmd, ciConfig.Steps[0].Dependencies)
	res.Response = "OK"
	return nil
}

func StartRunner(addr string) error {
	quit := make(chan interface{})
	done := make(chan interface{})
	listener, err := net.Listen("tcp", addr)
	runnerProxy := &Runner{}
	rpcServer := rpc.NewServer()

	// Publish Runner proxy object
	rpcServer.RegisterName("Runner", runnerProxy)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Listening on %v\n", listener.Addr())

	// Wait for incoming connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-quit:
					listener.Close()
					close(done)
					return
				default:
					log.Fatal(err)
				}
			}
			log.Print("Connection accepted")
			go func() {
				rpcServer.ServeConn(conn)
			}()
		}
	}()

	<-done
	return nil
}
