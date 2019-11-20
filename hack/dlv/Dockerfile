FROM concourse/concourse:local

ENV EDITOR=vim

RUN apt install -y vim
RUN go get -u -v github.com/go-delve/delve/cmd/dlv

ENTRYPOINT [ "dlv" ]
