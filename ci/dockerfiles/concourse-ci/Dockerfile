FROM ubuntu:14.04

RUN apt-get update && \
    apt-get -y install git unzip ruby ruby-dev \
        libxml2-dev libxslt-dev libcurl4-openssl-dev \
        build-essential zlib1g-dev libssl-dev libreadline6-dev libyaml-dev

ADD https://github.com/cloudfoundry-incubator/spiff/releases/download/v1.0.5/spiff_linux_amd64.zip /tmp/
RUN unzip /tmp/spiff_linux_amd64.zip -d /usr/local/bin
RUN rm /tmp/spiff_linux_amd64.zip

RUN gem install bundler --no-rdoc --no-ri
RUN gem install bosh_cli -v 1.2915.0 --no-rdoc --no-ri
