FROM ubuntu:15.10

# The Basics
RUN apt-get -y update && apt-get -y install build-essential curl

# Go
RUN curl https://storage.googleapis.com/golang/go1.5.1.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# SSH Client for TSA
RUN apt-get -y update && apt-get -y install openssh-client
