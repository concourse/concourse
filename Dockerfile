FROM selenium/standalone-firefox

USER root

# The Basics
RUN apt-get update && apt-get -y install curl

# Go
ADD go*.tar.gz /usr/local
ENV PATH $PATH:/usr/local/go/bin

# PostgreSQL
RUN apt-get update && apt-get -y install postgresql

# install selenium-driver wrapper binary for Agouti
RUN echo '#!/bin/sh' >> /usr/local/bin/selenium-server && \
    echo 'exec java -jar /opt/selenium/selenium-server-standalone.jar "$@" > /tmp/selenium.log 2>&1' >> /usr/local/bin/selenium-server && \
    chmod +x /usr/local/bin/selenium-server

RUN locale-gen en_US.UTF-8
ENV LANG en_US.UTF-8
