FROM ubuntu:16.04

RUN apt-get update && apt-get -y install git unzip

ADD bosh-cli-* /usr/local/bin/bosh
RUN chmod +x /usr/local/bin/bosh
