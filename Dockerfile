FROM concourse/concourse-ci

# The Basics
RUN apt-get update && apt-get -y install curl

# Go 1.3.1
RUN curl https://storage.googleapis.com/golang/go1.3.1.linux-amd64.tar.gz | \
      tar -C /usr/local -xzf -
