FROM ubuntu:xenial

ENV KUBE_LATEST_VERSION="v1.12.2"

RUN apt-get update \
 && apt-get install -y curl ca-certificates \
 && curl -L https://storage.googleapis.com/kubernetes-release/release/${KUBE_LATEST_VERSION}/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
 && chmod +x /usr/local/bin/kubectl

WORKDIR /root

RUN openssl genrsa -des3 -passout pass:x -out server.pass.key 2048 \
 && openssl rsa -passin pass:x -in server.pass.key -out server.key \
 && rm server.pass.key \
 && openssl req -new -key server.key -out server.csr -subj '/CN=pks.example.com/O=My Company Name LTD./C=US' \
 && openssl x509 -req -sha256 -days 365 -in server.csr -signkey server.key -out server.crt

COPY ./main ./pks-cluster-api
COPY ./cluster.yaml.tmpl .
COPY ./master.yaml.tmpl .

ENTRYPOINT [ "./pks-cluster-api" ]
