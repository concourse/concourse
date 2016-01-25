FROM rubinius/docker

RUN locale-gen en_US.UTF-8
RUN dpkg-reconfigure locales
ENV LANG en_US.UTF-8
ENV LC_ALL en_US.UTF-8

RUN gem install anatomy -v 0.1.7
RUN anatomy --version
