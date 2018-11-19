FROM concourse/dev

RUN set -x && \
  apt-get update && \
  apt-get install -y apt-transport-https && \
  curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
  touch /etc/apt/sources.list.d/kubernetes.list && \
  echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list && \
  apt-get update && \
  apt-get install -y kubectl jq && \
  curl https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get | bash
