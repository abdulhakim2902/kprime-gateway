FROM golang:alpine AS builder

RUN mkdir /src
ADD . /src
WORKDIR /src

RUN CGO_ENABLED=0 GOOS=linux go build -o gateway main.go

FROM alpine:latest AS final

RUN apk add --no-cache tzdata
ENV TZ=Asia/Singapore

COPY --from=builder /src/gateway .
COPY --from=builder /src/internal/fix-acceptor/config/FIX44.xml .

RUN mkdir -p /config/json
COPY --from=builder /src/datasources/json/supported-currencies.json /config/json

CMD ["./gateway"]