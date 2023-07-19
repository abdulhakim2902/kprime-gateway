FROM alpine:latest

RUN apk add --no-cache tzdata
ENV TZ=Asia/Singapore

WORKDIR /

COPY ./gateway /gateway
COPY ./internal/fix-acceptor/config/FIX44.xml /FIX44.xml

RUN mkdir -p /config/json
COPY ./datasources/json/supported-currencies.json /config/json/supported-currencies.json

CMD ["./gateway"]