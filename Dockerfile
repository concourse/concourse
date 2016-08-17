FROM golang:1

# install Go BOSH CLI
ADD https://s3.amazonaws.com/dk-shared-assets/cli-linux-feb11 /usr/local/bin/bosh
RUN chmod +x /usr/local/bin/bosh

# directory in which to place prebuilt assets vendored by the concourse binary
RUN mkdir /opt/static-assets

# pre-build `tar` so we don't have to every time
RUN cd /tmp && curl https://ftp.gnu.org/gnu/tar/tar-1.28.tar.gz | tar zxf - && \
      cd tar-1.28 && \
        FORCE_UNSAFE_CONFIGURE=1 ./configure && \
        make LDFLAGS=-static && \
        cp src/tar /opt/static-assets/tar && \
      cd .. && \
      rm -rf tar-1.28

# install pkg-config for building btrfs-progs and runc with seccomp
RUN apt-get update && \
      apt-get -y install pkg-config

# pre-build `iptables`
RUN apt-get update && \
      apt-get -y install bzip2 && \
      cd /tmp && curl http://www.netfilter.org/projects/iptables/files/iptables-1.4.21.tar.bz2 | tar jxf - && \
      cd iptables-1.4.21 && \
        mkdir /opt/static-assets/iptables && \
        ./configure --prefix=/opt/static-assets/iptables --enable-static --disable-shared && \
        make && \
        make install && \
      cd .. && \
      rm -rf iptables-1.4.21

# pre-build btrfs-progs
RUN apt-get update && \
      apt-get -y install liblzo2-dev libblkid-dev e2fslibs-dev libz-dev && \
      cd /tmp && \
      curl https://www.kernel.org/pub/linux/kernel/people/kdave/btrfs-progs/btrfs-progs-v4.4.tar.gz | tar zxf - && \
      cd btrfs-progs-v4.4 && \
      LDFLAGS=-static ./configure --disable-documentation && \
      make && \
      cp btrfs mkfs.btrfs /opt/static-assets && \
      cd /tmp && \
      rm -rf btrfs-progs-v4.4 && \
      apt-get -y remove liblzo2-dev libblkid-dev e2fslibs-dev libz-dev

# pre-build libseccomp
RUN cd /tmp && \
      curl -L https://github.com/seccomp/libseccomp/releases/download/v2.3.1/libseccomp-2.3.1.tar.gz | tar zxf - && \
      cd libseccomp-2.3.1 && \
        ./configure --prefix=/opt/static-assets/libseccomp && \
        make && \
        make install && \
      cd /tmp && \
      rm -rf libseccomp-2.3.1
