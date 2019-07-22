FROM srcd/dind-golang:docker-18.09.7-go-1.12.7

RUN apk update && apk upgrade && \
    apk add --no-cache bash git build-base

COPY build/bin/bblfsh-performance /root/
WORKDIR /root

ENTRYPOINT ["./bblfsh-performance"]
