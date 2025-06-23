## Running The `worker/runtime/integration` Test Suite

This test suite only works on Linux. If you're on macOS or Windows you can use
docker to run the Linux parts of the codebase. Use the following commands to
run this test suite:

```bash
docker run -v ~/workspace/concourse:/src -it --privileged --entrypoint "/bin/bash" concourse/dev
```

The above command will put you in a terminal session inside a container with
your local Concourse code mounted at `/src`.

To run the tests:

```bash
cd /src
go test -v ./worker/runtime/integration
```

You can leave the container running while you modify your code. Return to the
container whenever you want to run your tests to see if your changes work.
