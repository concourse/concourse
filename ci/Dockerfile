FROM ubuntu:14.04

RUN apt-get -y install git

RUN apt-get -y install unzip

ADD https://github.com/cloudfoundry-incubator/spiff/releases/download/v1.0/spiff_linux_amd64.zip /tmp/
RUN unzip /tmp/spiff_linux_amd64.zip -d /usr/local/bin
RUN rm /tmp/spiff_linux_amd64.zip

RUN apt-get -y install ruby ruby-dev

# bosh_cli (nokogiri)
RUN apt-get -y install libxml2-dev libxslt-dev libcurl4-openssl-dev

RUN apt-get -y install build-essential zlib1g-dev libssl-dev libreadline6-dev libyaml-dev 

RUN gem install bundler --no-rdoc --no-ri

RUN gem install bosh_cli --no-rdoc --no-ri
