ARG postgres_image=postgres
FROM ${postgres_image} AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    ca-certificates

RUN curl -L https://github.com/wolfcw/libfaketime/archive/refs/tags/v0.9.10.tar.gz --output faketime.tar.gz && \
    tar xzf faketime.tar.gz && cd libfaketime* && \
    make && make install && \
    cd .. && rm faketime.tar.gz && rm -rf libfaketime*

# Have to set locale due to postgres 16 recent update
RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    echo 'LANG="en_US.UTF-8"'>/etc/default/locale && \
    dpkg-reconfigure --frontend=noninteractive locales

# Have to preload faketime to work with postgres.
# https://github.com/wolfcw/libfaketime/issues/267
RUN sed -i -e '315i\export LD_PRELOAD=/usr/local/lib/faketime/libfaketime.so.1'   /usr/local/bin/docker-entrypoint.sh
