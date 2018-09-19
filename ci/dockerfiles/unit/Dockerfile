FROM golang:1

# basic setup for adding new apt sources
RUN apt-get update && apt-get -y install apt-transport-https

# install PostgreSQL for DB tests
RUN apt-get update && apt-get -y install postgresql
ENV PATH $PATH:/usr/lib/postgresql/9.6/bin

# install Node 8.x
RUN curl -sL https://deb.nodesource.com/setup_8.x | bash -
RUN apt-get update && apt-get install -y nodejs

# install Yarn for web UI tests
RUN curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add -
RUN echo "deb https://dl.yarnpkg.com/debian/ stable main" | tee /etc/apt/sources.list.d/yarn.list
RUN apt-get update && apt-get -y install yarn
