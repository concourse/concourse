FROM concourse/concourse:local

RUN go get -u -v github.com/go-delve/delve/cmd/dlv

ENTRYPOINT [ "dlv" ]
