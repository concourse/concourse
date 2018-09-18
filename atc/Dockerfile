FROM ubuntu:17.10

# The Basics
RUN apt-get update && apt-get -y install curl

# install PhantomJS 2.1.1
RUN apt-get update && apt-get -y install libfontconfig bzip2
RUN curl -L https://bitbucket.org/ariya/phantomjs/downloads/phantomjs-2.1.1-linux-x86_64.tar.bz2 | tar -C /tmp -jxf - && \
      mv /tmp/phantomjs-*/bin/phantomjs /usr/local/bin && \
      rm -rf /tmp/phantomjs-*

# Go, with build-essential for gcc
RUN apt-get update && apt-get -y install build-essential
ADD go*.tar.gz /usr/local
ENV PATH $PATH:/usr/local/go/bin

# Git for `go get` in pull request task
RUN apt-get update && apt-get -y install git

# PostgreSQL
RUN apt-get update && apt-get -y install postgresql-9.6
ENV PATH $PATH:/usr/lib/postgresql/9.6/bin
