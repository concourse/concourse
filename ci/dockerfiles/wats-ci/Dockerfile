FROM ruby:2

# ChromeDriver
RUN apt-get update && apt-get -y install xvfb chromedriver
ENV PATH $PATH:/usr/lib/chromium

# Go, with build-essential for gcc
RUN apt-get update && apt-get -y install build-essential
ADD go*.tar.gz /usr/local
ENV PATH $PATH:/usr/local/go/bin
