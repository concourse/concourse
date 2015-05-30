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

ADD https://github.com/vito/atomy/archive/works.zip /tmp/atomy.zip

ADD https://github.com/vito/broomhlda/archive/master.zip /tmp/broomhlda.zip

ADD https://github.com/vito/anatomy/archive/master.zip /tmp/anatomy.zip

RUN cd /tmp && unzip atomy.zip && rm atomy.zip && cd atomy-works && \
    gem build atomy.gemspec && gem install atomy-*.gem && \
    cd .. && rm -rf atomy-works

RUN cd /tmp && unzip broomhlda.zip && rm broomhlda.zip && cd broomhlda-master && \
    gem build broomhlda.gemspec && gem install broomhlda-*.gem && \
    cd .. && rm -rf broomhlda-master

RUN cd /tmp && unzip anatomy.zip && rm anatomy.zip && cd anatomy-master && \
    gem build anatomy.gemspec && gem install anatomy-*.gem && \
    cd .. && rm -rf anatomy-master
