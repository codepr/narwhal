Narwhal
=======

![Go](https://github.com/codepr/narwhal/workflows/Go/badge.svg)

PoC of a very simple CI system consists of 2 processes:

- The dispatcher, a simple RESTful server exposing some APIs to submit commits
  and register new runners
- The runner, a RESTful server as well that run tests (and arguably other
  instructions in the future) safely inside an isolated environment by creating
  containers on-the-go.

### Rationale

First project in Go, actually made to learn the language as it offers a lot of
space for improvements and incremental addition of features.

Ideally a bunch of runners should be spread on a peer’s subnet with similar hw
and each one registers itself to the dispatcher. Beside registering itself,
another way could very well be to use a load-balancer or a proxy, registering
it’s URL to the dispatcher and demanding the job distributions to it.

### Roadmap

As said, it's a project that spans multiple layers and very keen to useful
features addition, here's a list of just some of the ideas that comes to mind,
to be expanded probably:

- Core
    - Dispatcher RESTful server
        - `/commit` to manage testrunner servers, this endpoint will be used to
          register new runners (POST) and to get a list of them as well (GET)
        - `/runner` to add new commits to be processed, each commit is comprised of
          its ID and the repository it belongs to. Future development will wrap
          this abstraction on a more generic "Job" with some stats and tracking of
          the progress status.
        - `/stats` retrieve some stats of sort like number of commits processed
          per repository, % completed/failed jobs etc.

    - Runner RESTful server
        - `/health` return the status of health (e.g. the reachability) of the
          runner server
        - `/commit` a commit that needs to be processed, each commit received it's
          assumed to not be processed yet. Processing of a commit consists in the
          creation of a container (Docker as main system) and execution of
          arbitrary commands inside.
        - `/stats` retrieve some stats of sort like number of commits processed
          per repository, % completed/failed jobs etc.

- Optional
    - Results channels front-end friendly, maybe through websockets given the
      nature of the executed tests on `stdout` and `stderr` streams
