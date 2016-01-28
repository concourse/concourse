FROM ubuntu:15.10

# The Basics
RUN apt-get update && apt-get -y install curl

# Go
RUN curl https://storage.googleapis.com/golang/go1.5.3.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH $PATH:/usr/local/go/bin

# PostgreSQL
RUN apt-get update && apt-get -y install postgresql

# NPM (legacy package provides 'node' binary which many npm packages need)
RUN apt-get update && apt-get -y install nodejs-legacy

# Git (for elm-package)
RUN apt-get update && apt-get -y install git

# install PhantomJS 2.1.1
RUN apt-get update && apt-get -y install libfontconfig
RUN curl -L https://bitbucket.org/ariya/phantomjs/downloads/phantomjs-2.1.1-linux-x86_64.tar.bz2 | tar -C /tmp -jxf - && \
      mv /tmp/phantomjs-*/bin/phantomjs /usr/local/bin && \
      rm -rf /tmp/phantomjs-*

RUN locale-gen en_US.UTF-8
ENV LANG en_US.UTF-8
