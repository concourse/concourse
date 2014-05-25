FROM mischief/docker-golang

RUN \
  cd /tmp && \
    wget http://download.redis.io/redis-stable.tar.gz && \
    tar xvzf redis-stable.tar.gz && \
    cd redis-stable && \
    make && \
    make install && \
    rm -rf /tmp/redis-stable*
