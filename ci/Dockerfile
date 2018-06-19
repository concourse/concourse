FROM concourse/bin-ci

# postges
RUN apt-get update && apt-get -y install postgresql-9.6
ENV PATH $PATH:/usr/lib/postgresql/9.6/bin

# ruby
RUN apt-get update && apt-get -y install ruby ruby-dev
RUN gem install bundler

# ChromeDriver
RUN apt-get update && apt-get -y install xvfb chromedriver
ENV PATH $PATH:/usr/lib/chromium

RUN apt-get -y update && apt-get -y install netcat
