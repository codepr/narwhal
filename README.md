Narwhal
=======

![Go](https://github.com/codepr/narwhal/workflows/Go/badge.svg)

PoC of a basic continous integration local server. Currently consists of 2
processes:

- Dispatcher, listening for new commit requests, currently exposes just 2 REST
  APIs:
  - `/runner` to manage testrunner servers, this endpoint will be used to
    register new runners (POST) and to get a list of them as well (GET)
  - `/commit` to add new commits to be processed, each commit is comprised of
    its ID and the repository it belongs to. Future development will wrap this
    abstraction on a more generic "Job" with some stats and tracking of the
    progress status.

- TestRunner, the worker, should be 1 or more, ideally spread on a peer's subnet.
  Each testrunner must register to the dispatcher in order to receive jobs, and
  orchestrate a pool of containers to run tests (and arguably other instructions).
  Beside registering to the dispatcher, another way could very well be to use a
  load-balancer or a proxy, registering it's URL to the dispatcher and
  demanding the job distributions to it.

First project in Go, actually made to learn the language as it offers a lot of
space for improvements and incremental addition of features.
