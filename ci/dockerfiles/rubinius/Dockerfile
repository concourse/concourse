FROM ubuntu:15.04

RUN apt-get -y update && apt-get -y install build-essential bison ruby \
      ruby-dev rake zlib1g-dev libyaml-dev libssl-dev libreadline-dev \
      libncurses5-dev llvm-3.5 llvm-3.5-dev libeditline-dev libedit-dev

RUN gem install bundler

RUN apt-get -y install wget && \
    wget http://releases.rubini.us/rubinius-2.5.5.tar.bz2 \
         -O /tmp/rubinius.tar.bz2

RUN locale-gen en_US.UTF-8
RUN dpkg-reconfigure locales
ENV LANG en_US.UTF-8
ENV LC_ALL en_US.UTF-8

RUN cd /tmp && tar jxf rubinius.tar.bz2 && rm *.tar.bz2 && cd rubinius-* && \
    bundle install && \
    ./configure --prefix=/opt/rubinius \
                --llvm-config=/usr/bin/llvm-config-3.5 && \
    rake build && \
    rake install && \
    cd /tmp && rm -rf rubinius-*

ENV PATH /opt/rubinius/bin:/opt/rubinius/gems/bin:$PATH
