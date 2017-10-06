FROM ruby:2

RUN apt-get update && apt-get -y install sudo unzip libnss3 libgconf-2-4

# ChromeDriver
RUN wget -N http://chromedriver.storage.googleapis.com/2.32/chromedriver_linux64.zip \
&& unzip chromedriver_linux64.zip \
&& chmod +x chromedriver \
&& sudo mv -f chromedriver /usr/local/share/chromedriver \
&& sudo ln -s /usr/local/share/chromedriver /usr/local/bin/chromedriver \
&& sudo ln -s /usr/local/share/chromedriver /usr/bin/chromedriver

# Chromium
RUN apt-get update && apt-get -y install chromium
ENV PATH $PATH:/usr/lib/chromium

# Go, with build-essential for gcc
RUN apt-get update && apt-get -y install build-essential
ADD go*.tar.gz /usr/local
ENV PATH $PATH:/usr/local/go/bin
