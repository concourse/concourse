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

# NPM
RUN curl -sL https://deb.nodesource.com/setup_4.x | bash -
RUN apt-get -y install nodejs
RUN update-alternatives --install /usr/bin/node node /usr/bin/nodejs 10

# Git (for elm-package)
RUN apt-get -y install git

RUN locale-gen en_US.UTF-8
ENV LANG en_US.UTF-8
