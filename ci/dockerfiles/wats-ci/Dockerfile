FROM ruby:2

RUN apt-get update && apt-get -y install sudo unzip libnss3 libgconf-2-4

# ChromeDriver
RUN wget -N http://chromedriver.storage.googleapis.com/2.32/chromedriver_linux64.zip
RUN unzip chromedriver_linux64.zip
RUN chmod +x chromedriver
RUN sudo mv -f chromedriver /usr/local/share/chromedriver
RUN sudo ln -s /usr/local/share/chromedriver /usr/local/bin/chromedriver
RUN sudo ln -s /usr/local/share/chromedriver /usr/bin/chromedriver

# Go, with build-essential for gcc
RUN apt-get update && apt-get -y install build-essential
ADD go*.tar.gz /usr/local
ENV PATH $PATH:/usr/local/go/bin
