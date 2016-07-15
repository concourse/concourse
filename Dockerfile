FROM ubuntu:16.04

# The Basics
RUN apt-get -y update && apt-get -y install build-essential curl

# Go
RUN curl https://storage.googleapis.com/golang/go1.6.1.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# SSH Client for TSA
RUN apt-get -y update && apt-get -y install openssh-client
