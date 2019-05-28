FROM golang:1

# basic setup for adding new apt sources and installing packages
RUN apt-get update && apt-get -y install \
      apt-transport-https \
      ca-certificates \
      curl \
      gnupg2 \
      software-properties-common \
      unzip

# install PostgreSQL for DB tests
RUN curl https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
RUN add-apt-repository "deb http://apt.postgresql.org/pub/repos/apt/ stretch-pgdg main"
RUN apt-get update && apt-get -y install postgresql-11
ENV PATH $PATH:/usr/lib/postgresql/11/bin

# install Node 8.x
RUN curl -sL https://deb.nodesource.com/setup_8.x | bash -
RUN apt-get update && apt-get install -y nodejs

# install Yarn for web UI tests
RUN curl -fsSL https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add -
RUN add-apt-repository "deb https://dl.yarnpkg.com/debian/ stable main"
RUN apt-get update && apt-get -y install yarn

# install Docker
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add -
RUN add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable"
RUN apt-get update && apt-get -y install docker-ce

# install Docker Compose
RUN curl -L "https://github.com/docker/compose/releases/download/1.22.0/docker-compose-$(uname -s)-$(uname -m)" \
      -o /usr/local/bin/docker-compose && \
      chmod +x /usr/local/bin/docker-compose

# install Chrome for Puppeteer (watsjs)
RUN curl -fsSL https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add -
RUN add-apt-repository "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main"
RUN apt-get update && apt-get -y install google-chrome-unstable --no-install-recommends

# install BOSH CLI for bosh-smoke, bosh-topgun
RUN curl -L "https://github.com/cloudfoundry/bosh-cli/releases/download/v5.3.1/bosh-cli-5.3.1-linux-amd64" \
      -o /usr/local/bin/bosh && \
      chmod +x /usr/local/bin/bosh

# install kubectl for k8s-related jobs
RUN curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
      touch /etc/apt/sources.list.d/kubernetes.list && \
      echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list && \
      apt-get update && \
      apt-get install --no-install-recommends -y kubectl && \
      rm -rf /var/lib/apt/lists/*

# install helm for k8s-related jobs
RUN curl https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get | bash

# install Vault for bosh-topgun
RUN curl -L https://releases.hashicorp.com/vault/0.7.3/vault_0.7.3_linux_amd64.zip -o /tmp/vault.zip && \
      unzip /tmp/vault.zip -d /usr/local/bin && \
      rm /tmp/vault.zip

# install Terraform and jq for bin-smoke
RUN curl -fsSL https://releases.hashicorp.com/terraform/0.12.0/terraform_0.12.0_linux_amd64.zip -o /tmp/terraform.zip && \
      unzip /tmp/terraform.zip -d /usr/local/bin && \
      rm /tmp/terraform.zip
RUN apt-get update && apt-get -y install jq
