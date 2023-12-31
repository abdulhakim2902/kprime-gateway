FROM golang:alpine AS builder

ARG ACCESS_USER
ARG ACCESS_TOKEN
ARG UTILITIES

ENV GOPRIVATE=github.com/Undercurrent-Technologies/kprime-utilities

RUN apk add git

RUN git config --global url.https://${ACCESS_USER}:${ACCESS_TOKEN}@github.com/.insteadOf https://github.com

RUN mkdir /src
ADD . /src
WORKDIR /src

# RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o gateway main.go

FROM alpine:latest AS final

ARG UTILITIES

RUN apk add --no-cache tzdata
ENV TZ=Asia/Singapore
ENV IMAGE=true

COPY --from=builder /src/gateway .
COPY --from=builder /src/internal/fix-acceptor/config/FIX44.xml .

RUN mkdir -p /config/json
COPY --from=builder /src/${UTILITIES}/config/json/supported-currencies.json /config/json

CMD ["./gateway"]