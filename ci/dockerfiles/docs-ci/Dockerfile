FROM concourse/rubinius

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
