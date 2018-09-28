FROM golang:1

# basic setup for adding new apt sources
RUN apt-get update && apt-get -y install \
      apt-transport-https \
      ca-certificates \
      curl \
      gnupg2 \
      software-properties-common

# install PostgreSQL for DB tests
RUN apt-get update && apt-get -y install postgresql
ENV PATH $PATH:/usr/lib/postgresql/9.6/bin

# install Node 8.x
RUN curl -sL https://deb.nodesource.com/setup_8.x | bash -
RUN apt-get update && apt-get install -y nodejs

# install Yarn for web UI tests
RUN curl -fsSL https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add -
RUN add-apt-repository "deb https://dl.yarnpkg.com/debian/ stable main"
RUN apt-get update && apt-get -y install yarn

# install Docker
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add -
RUN add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable"
RUN apt-get update && apt-get -y install docker-ce

# install Docker Compose
RUN curl -L "https://github.com/docker/compose/releases/download/1.22.0/docker-compose-$(uname -s)-$(uname -m)" \
      -o /usr/local/bin/docker-compose && \
      chmod +x /usr/local/bin/docker-compose

# install Chrome for Puppeteer
RUN curl -fsSL https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add -
RUN add-apt-repository "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main"
RUN apt-get update && && apt-get -y install google-chrome-unstable --no-install-recommends
