Narwhal
=======

PoC of a basic continous integration local server. Currently consists of 2
processes:

- Dispatcher, listening for new commit requests
- TestRunner, the worker, should be 1 or more, ideally spread on a peer's subnet.
  Each testrunner must register to the dispatcher in order to receive jobs, and
  orchestrate a pool of containers to run tests (and arguably other instructions).
  Beside registering to the dispatcher, another way could very well be to use a
  load-balancer or a proxy, registering it's URL to the dispatcher and
  demanding the job distributions to it.

First project in Go, actually made to learn the language as it offers a lot of
space for improvements and incremental addition of features.
