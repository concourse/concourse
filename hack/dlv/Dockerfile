FROM concourse/concourse:local

RUN go get -u -v github.com/derekparker/delve/cmd/dlv

ENTRYPOINT [ "dlv" ]
