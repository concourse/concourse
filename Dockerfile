FROM node

RUN npm install -g phantomjs

RUN apt-get update && apt-get -y install locales && locale-gen en_US.UTF-8
ENV LANG en_US.UTF-8
