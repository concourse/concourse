# This Dockerfile is only for running integration tests that depend on a running containerd process
# Not intended to be packaged or used for production

FROM concourse/golang-builder

RUN mkdir -p /go/src/github.com/concourse/ /usr/local/concourse/bin/
ENV PATH=/usr/local/concourse/bin:$PATH

COPY start.sh /usr/local/concourse/bin/start.sh
VOLUME /go/src/github.com/concourse/concourse
VOLUME /go

ARG RUNC_VERSION=v1.0.0-rc8
ARG CNI_VERSION=v0.8.2
ARG CONTAINERD_VERSION=1.3.0

RUN apt update && apt install -y curl vim

RUN curl -sSL https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64 -o /usr/local/concourse/bin/init && chmod +x /usr/local/concourse/bin/init
RUN set -x && \
	curl -sSL https://github.com/containerd/containerd/releases/download/v$CONTAINERD_VERSION/containerd-$CONTAINERD_VERSION.linux-amd64.tar.gz \
		| tar -zvxf - -C /usr/local/concourse/bin --strip-components=1 && \
	curl -sSL https://github.com/opencontainers/runc/releases/download/$RUNC_VERSION/runc.amd64 \
		-o /usr/local/concourse/bin/runc && chmod +x /usr/local/concourse/bin/runc && \
	curl -sSL https://github.com/containernetworking/plugins/releases/download/$CNI_VERSION/cni-plugins-linux-amd64-$CNI_VERSION.tgz \
		| tar -zvxf - -C /usr/local/concourse/bin

ENTRYPOINT ["/usr/local/concourse/bin/init", "/usr/local/concourse/bin/start.sh"]

