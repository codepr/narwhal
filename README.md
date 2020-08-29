Narwhal
=======

![Go](https://github.com/codepr/narwhal/workflows/Go/badge.svg)

PoC of a very simple CI system consists of 3 microservices:

- **Agent:** It's the watcher process, ideally it should subscribe to remote
  repositories (e.g. webhooks on github) waiting for new events to be
  dispatched to the workers asynchronously through a middleware, for example a
  RabbitMQ task-queue.

- **Dispatcher:** a simple RESTful server, responsible for load-balancing CI
  jobs to a pool of workers (runner) through RPC (currently using built-in
  `net/rpc` package, `gRPC` would probably be a better solution for production)
  and collecting some useful stats by monitoring their state. Exposes some APIs
  to get the job's related infos or to force some re-submit.

- **Runner:** Orchestrate received jobs safely inside an isolated environment
  by creating containers on-the-go.

### Rationale

Simple project in Go, actually made to learn the language as it offers a lot of
space for improvements and incremental addition of features.

Ideally a bunch of runners should be spread on a peer’s subnet with similar hw
and each one registers itself to the dispatcher. Beside registering itself,
another way could very well be to use a load-balancer or a proxy, registering
it’s URL to the dispatcher and demanding the job distributions to it.
