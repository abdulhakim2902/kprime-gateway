# KPrime - Gateway

[![dev](https://img.shields.io/badge/development-ok-brightgreen)](https://k-gateway.devucc.name/)

Gateway in KPrime serves as a bridge between different systems, enabling seamless communication and integration. It acts as a central hub, facilitating the exchange of data and messages between various components within the trading ecosystem. REST API, WebSocket, and FIX protocol are used as transport layers in Gateway.

Please see [wiki](https://git.devucc.name/groups/undercurrent-tech/k-prime/-/wikis/home) for all other important notes.

## Table of Contents

- [Prerequisite](#prerequisite)
- [Install](#install)
- [Docker](#docker)
- [Metrics](#metrics)

## Prerequisite

Make sure you have Go installed and GOPATH set in your local.
```
Go version: 1.20.3 
MongoDB version: Community Edition 6.0.5
```

And we need 2 other applications + 1 seeding (if you havent done seeding):
- [MongoDB](https://git.devucc.name/groups/undercurrent-tech/k-prime/-/wikis/Docker-%F0%9F%90%B3#1-mongodb)
- [Apache Kafka](https://git.devucc.name/groups/undercurrent-tech/k-prime/-/wikis/Docker-%F0%9F%90%B3#2-kafka) 
- [Seeding](https://git.devucc.name/groups/undercurrent-tech/k-prime/-/wikis/Docker-%F0%9F%90%B3#4-user-seeding)

This projects uses Gin and implements Clean Architecture

- for gin related docs please visit https://gin-gonic.com/docs/

- for clean architecture concepts you can visit the link: https://evrone.com/go-clean-template

## Install

copy file .env.example and rename it to .env, modify .env file based on your local requirement.
```
cp .env.example .env
```

To be able to get private dependencies, you can use this following command:
```
GOPRIVATE=git.devucc.name/dependencies/utilities go get git.devucc.name/dependencies/utilities@v1.0.53
```

To run the application for development purposes:
```
go run main.go
```

For production purpose:
```
go build -o main main.go
./main
```

This project uses [node](http://nodejs.org) and [npm](https://npmjs.com). Go check them out if you don't have them locally installed.

```sh
$ npm install --global standard-readme-spec
```

Swagger Documentation. 
To start generating documentation for an endpoint, you can start with decorating the controller/handler with annotations. You can do this by adding commented text before the handler func. 
```// @BasePath /api/internal

// Sync memdb with mongodb godoc
// @Summary Sync memdb with mongodb
// @Schemes
// @Description do sync
// @Tags internal
// @Accept json
// @Produce json
// @Success 200 {string} ok
// @Router /sync/:target [post]
```

when you've done the decorating, you can then generate the actual yaml and json swagger by running this command 
```
swag init -g path/to/handler-file.go --output docs/internal
```

the docs can be accessed in {{url}}/swagger/index.html
## Docker

To run gateway through the docker, you can follow this following [wiki](https://git.devucc.name/groups/undercurrent-tech/k-prime/-/wikis/Docker-%F0%9F%90%B3#app-gateway)

To build and push the docker image in this [wiki](https://git.devucc.name/groups/undercurrent-tech/k-prime/-/wikis/Docker-%F0%9F%90%B3#building-and-publishing-docker)

### Metrics

Run in port 2112 (default), Metrics endpoint:
```
<host>:2112/metrics
```
Custom port with env variable:
```
METRICS_PORT="2113"
```

