FROM concourse/atc-ci

# install Vault
RUN apt-get update && apt-get -y install unzip && \
      curl -L https://releases.hashicorp.com/vault/0.7.3/vault_0.7.3_linux_amd64.zip -o /tmp/vault.zip && \
      unzip /tmp/vault.zip -d /usr/local/bin && \
      rm /tmp/vault.zip && \
      apt-get -y remove unzip
