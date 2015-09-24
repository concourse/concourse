FROM ubuntu:14.04

# The Basics
RUN apt-get update && apt-get -y install build-essential curl

# Go
RUN echo "deb http://mirror.anl.gov/pub/ubuntu trusty main universe" >> /etc/apt/sources.list
RUN curl https://storage.googleapis.com/golang/go1.5.1.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH $PATH:/usr/local/go/bin

# PostgreSQL
RUN apt-get -y install postgresql-9.3
RUN chmod 0777 /var/run/postgresql

# PhantomJS
RUN apt-get -y install build-essential chrpath libssl-dev libxft-dev \
  libfreetype6 libfreetype6-dev libfontconfig1 libfontconfig1-dev unzip \
  libjpeg8 libicu52

ADD https://s3-us-west-1.amazonaws.com/concourse-public/phantomjs-2.0.0-20141016-u1404-x86_64.zip /tmp/phantomjs-2.0.0-20141016-u1404-x86_64.zip
RUN cd /tmp && unzip phantomjs-2.0.0-20141016-u1404-x86_64.zip && rm /tmp/phantomjs-2.0.0-20141016-u1404-x86_64.zip

RUN mv /tmp/phantomjs-2.0.0-20141016 /usr/local/share
RUN ln -sf /usr/local/share/phantomjs-2.0.0-20141016/bin/phantomjs /usr/local/bin

# NPM
RUN apt-get -y install nodejs npm
RUN update-alternatives --install /usr/bin/node node /usr/bin/nodejs 10
