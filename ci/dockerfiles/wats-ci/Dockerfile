FROM ruby:2

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

# pre-install nokogiri to shorten build time
RUN gem install nokogiri
