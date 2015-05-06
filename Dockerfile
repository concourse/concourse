FROM concourse/concourse-ci

# The Basics
RUN apt-get update && apt-get -y install curl

# Go 1.4.2
RUN curl https://storage.googleapis.com/golang/go1.4.2.linux-amd64.tar.gz | \
      tar -C /usr/local -xzf -

ENV GOROOT /usr/local/go
ENV PATH $PATH:/usr/local/go/bin
