FROM ubuntu:14.04

# The Basics
RUN apt-get update && apt-get -y install build-essential curl

# Go 1.3
RUN echo "deb http://mirror.anl.gov/pub/ubuntu trusty main universe" >> /etc/apt/sources.list
RUN curl https://storage.googleapis.com/golang/go1.3.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# PostgreSQL
RUN apt-get -y install postgresql-9.3
RUN chmod 0777 /var/run/postgresql
