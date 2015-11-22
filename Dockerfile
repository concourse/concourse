FROM ubuntu:15.10

# The Basics
RUN apt-get update && apt-get -y install curl

# Go
RUN echo "deb http://mirror.anl.gov/pub/ubuntu trusty main universe" >> /etc/apt/sources.list
RUN curl https://storage.googleapis.com/golang/go1.5.1.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH $PATH:/usr/local/go/bin

# PostgreSQL
RUN apt-get -y install postgresql-9.4
RUN chmod 0777 /var/run/postgresql

# PhantomJS
RUN apt-get -y install unzip gcc-4.9 g++-4.9 make libc-dev flex bison gperf \
    ruby perl libsqlite3-dev libfontconfig1-dev libicu-dev libfreetype6 \
    libssl-dev libpng-dev libjpeg-dev python libx11-dev libxext-dev

ADD https://bitbucket.org/ariya/phantomjs/downloads/phantomjs-2.0.0-source.zip /tmp/phantomjs.zip

RUN cd /tmp && unzip phantomjs.zip && rm phantomjs.zip && \
    cd /tmp/phantomjs* && ./build.sh --confirm && cp bin/phantomjs /usr/local/bin && \
    cd /tmp && rm -rf phantomjs*

# NPM
RUN apt-get -y install nodejs npm
RUN update-alternatives --install /usr/bin/node node /usr/bin/nodejs 10
