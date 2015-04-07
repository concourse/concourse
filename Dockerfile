FROM ubuntu:14.04

# The Basics
RUN apt-get update && apt-get -y install build-essential curl

# Go 1.3
RUN echo "deb http://mirror.anl.gov/pub/ubuntu trusty main universe" >> /etc/apt/sources.list
RUN curl https://storage.googleapis.com/golang/go1.3.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# PostgreSQL
RUN apt-get -y install postgresql-9.3
RUN chmod 0777 /var/run/postgresql

# PhantomJS
RUN apt-get -y install build-essential chrpath libssl-dev libxft-dev \
  libfreetype6 libfreetype6-dev libfontconfig1 libfontconfig1-dev

ADD https://bitbucket.org/ariya/phantomjs/downloads/phantomjs-1.9.8-linux-x86_64.tar.bz2 /tmp/phantomjs-1.9.8-linux-x86_64.tar.bz2
RUN cd /tmp && tar xvjf phantomjs-1.9.8-linux-x86_64.tar.bz2 && rm /tmp/phantomjs-1.9.8-linux-x86_64.tar.bz2

RUN mv /tmp/phantomjs-1.9.8-linux-x86_64 /usr/local/share
RUN ln -sf /usr/local/share/phantomjs-1.9.8-linux-x86_64/bin/phantomjs /usr/local/bin
