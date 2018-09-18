FROM ubuntu:16.04

# The Basics
RUN apt-get -y update && apt-get -y install build-essential curl

# Go
ADD go*.tar.gz /usr/local
ENV PATH $PATH:/usr/local/go/bin

# SSH Client for TSA
RUN apt-get -y update && apt-get -y install openssh-client
