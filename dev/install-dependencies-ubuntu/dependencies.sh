#!/usr/bin/env bash
exec < /home/USER/concourse-ubuntu.log
set -x
set -e

# snap
sudo apt update
sudo apt install snapd

# yarn
sudo curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg |  apt-key add -
echo "deb https://dl.yarnpkg.com/debian/ stable main" |  tee /etc/apt/sources.list.d/yarn.list

# docker
sudo snap install docker

# docker-compose
sudo curl -L "https://github.com/docker/compose/releases/download/1.23.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose

# go-lang
sudo snap install go --classic

# Postgres
sudo snap install postgresql10

# install fly
go install ./fly

