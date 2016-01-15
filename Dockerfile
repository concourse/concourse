FROM robcherry/docker-chromedriver

# The Basics
RUN apt-get update && apt-get -y install curl

# Go
RUN curl https://storage.googleapis.com/golang/go1.5.3.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH $PATH:/usr/local/go/bin

# PostgreSQL
RUN echo "deb http://apt.postgresql.org/pub/repos/apt/ wheezy-pgdg main" >> /etc/apt/sources.list.d/pgdg.list
RUN curl -S https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
RUN apt-get update
RUN apt-get -y install postgresql-9.4
RUN chmod 0777 /var/run/postgresql

# PhantomJS
RUN apt-get -y install unzip build-essential gcc-4.7 g++-4.7 flex bison gperf \
    ruby perl libsqlite3-dev libfontconfig1-dev libicu-dev libfreetype6 \
    libssl-dev libpng-dev libjpeg-dev python libx11-dev libxext-dev

# must build with GCC <5
RUN update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-4.7 10
RUN update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-4.7 10

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
