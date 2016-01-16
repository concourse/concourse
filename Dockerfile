FROM node

RUN npm install -g phantomjs

RUN apt-get update && apt-get -y install locales && \
      echo 'en_US.UTF-8 UTF-8' >> /etc/locale.gen && \
      locale-gen

ENV LANG en_US.UTF-8
