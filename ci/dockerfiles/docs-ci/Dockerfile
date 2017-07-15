FROM golang:1

RUN go version
RUN apt-get update && apt-get -y install python3 python3-pip
RUN pip3 install pygments
RUN pygmentize -V

RUN go get github.com/vito/booklit/cmd/booklit
