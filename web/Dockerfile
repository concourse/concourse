FROM ubuntu:15.10

# The Basics
RUN apt-get update && apt-get -y install curl

# PhantomJS
RUN apt-get -y install unzip build-essential gcc-4.9 g++-4.9 flex bison gperf \
    ruby perl libsqlite3-dev libfontconfig1-dev libicu-dev libfreetype6 \
    libssl-dev libpng-dev libjpeg-dev python libx11-dev libxext-dev

# must build with GCC <5
RUN update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-4.9 10
RUN update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-4.9 10

ADD https://bitbucket.org/ariya/phantomjs/downloads/phantomjs-2.0.0-source.zip /tmp/phantomjs.zip

RUN cd /tmp && unzip phantomjs.zip && rm phantomjs.zip && \
    cd /tmp/phantomjs* && ./build.sh --confirm && cp bin/phantomjs /usr/local/bin && \
    cd /tmp && rm -rf phantomjs*

# NPM
RUN curl -sL https://deb.nodesource.com/setup_4.x | bash -
RUN apt-get -y install nodejs
RUN update-alternatives --install /usr/bin/node node /usr/bin/nodejs 10

# Git (for elm-package)
RUN apt-get -y install git

RUN locale-gen en_US.UTF-8
ENV LANG en_US.UTF-8
