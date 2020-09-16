FROM concourse/concourse:local

ENV GO111MODULE=off
ENV EDITOR=vim

RUN apt update && apt install -y vim
RUN go get -u -v github.com/go-delve/delve/cmd/dlv

ENTRYPOINT [ "dlv" ]
