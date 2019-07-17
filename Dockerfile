# TODO(lwsanty): change the base image when infra task #991 is done
FROM golang:1.12.7

RUN apt-get update
RUN apt-get -y install apt-transport-https ca-certificates curl gnupg2 software-properties-common
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add -
RUN add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable"

RUN apt-get update
RUN apt-get -y install docker-ce

COPY build/bin/bblfsh-performance /root/
WORKDIR /root

ENTRYPOINT ["./bblfsh-performance"]
