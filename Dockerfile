FROM golang:1

# install Go BOSH CLI
ADD https://s3.amazonaws.com/dk-shared-assets/cli-linux-feb11 /usr/local/bin/bosh
RUN chmod +x /usr/local/bin/bosh

# directory in which to place prebuilt assets vendored by the concourse binary
RUN mkdir /opt/static-assets

# pre-build `tar` so we don't have to every time
RUN cd /tmp && curl -L https://ftp.gnu.org/gnu/tar/tar-1.30.tar.gz | tar zxf - && \
      cd tar-* && \
        FORCE_UNSAFE_CONFIGURE=1 ./configure && \
        make LDFLAGS=-static && \
        cp src/tar /opt/static-assets/tar && \
      cd .. && \
      rm -rf tar-*

# install pkg-config for building btrfs-progs and runc with seccomp
RUN apt-get update && \
      apt-get -y install pkg-config

# pre-build `iptables` and dependencies
RUN set -x && \
      apt-get update && \
      apt-get -y install bzip2 file flex bison libzst-dev && \
      cd /tmp && \
      curl -L https://www.netfilter.org/projects/iptables/files/iptables-1.6.2.tar.bz2 | tar jxf - && \
      curl -L https://www.netfilter.org/projects/libmnl/files/libmnl-1.0.4.tar.bz2 | tar jxf - && \
      curl -L https://www.netfilter.org/projects/libnftnl/files/libnftnl-1.0.9.tar.bz2 | tar jxf - && \
      mkdir /opt/static-assets/iptables && \
      cd libmnl-* && \
        ./configure && \
        make && \
        make install && \
      cd .. && \
      cd libnftnl-* && \
        ./configure && \
        make && \
        make install && \
      cd .. && \
      cd iptables-* && \
        ./configure --prefix=/opt/static-assets/iptables --enable-static --disable-shared && \
        make && \
        make install && \
      cd .. && \
      rm -rf iptables-* \
      rm -rf libmnl-* \
      rm -rf libnftnl-*

# pre-build btrfs-progs
RUN set -x && \
      apt-get update && \
      apt-get -y install liblzo2-dev libblkid-dev e2fslibs-dev libz-dev && \
      cd /tmp && \
      curl -L https://www.kernel.org/pub/linux/kernel/people/kdave/btrfs-progs/btrfs-progs-v4.15.tar.gz | tar zxf - && \
      cd btrfs-progs-* && \
      LDFLAGS=-static ./configure --disable-documentation && \
      make && \
      cp btrfs mkfs.btrfs /opt/static-assets && \
      cd /tmp && \
      rm -rf btrfs-progs-* && \
      apt-get -y remove liblzo2-dev libblkid-dev e2fslibs-dev libz-dev

# pre-build libseccomp
RUN set -x && \
      cd /tmp && \
      curl -L https://github.com/seccomp/libseccomp/releases/download/v2.3.3/libseccomp-2.3.3.tar.gz | tar zxf - && \
      cd libseccomp-* && \
        ./configure --prefix=/opt/static-assets/libseccomp && \
        make && \
        make install && \
      cd /tmp && \
      rm -rf libseccomp-*
