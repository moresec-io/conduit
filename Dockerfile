FROM alpine:3.11

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk add tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
     && echo "Asia/Shanghai" > /etc/timezone \
RUN apk update -vU --allow-untrusted
RUN apk add --no-cache \
    bash

WORKDIR /app/bin

COPY release/conf /app/conf
COPY release/bin/conduit /app/bin

ENTRYPOINT [ "/app/bin/conduit", "-f", "/app/conf/conduit.yaml" ]
