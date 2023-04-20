## Options Exchange Gateway

### This projects uses Gin and implements Clean Architecture

for gin related docs please visit https://gin-gonic.com/docs/

for clean architecture concepts you can visit these links:

1. https://evrone.com/go-clean-template


### Setup the application

to setup the application, please copy file `.env.example` and rename it to `.env`
modify `.env` file based on your local requirement

### Database
We're using 2 databases. MySQL database is used only for gateway application, to store users' credentials. And MongoDB Database is connected to the orderbook's MongoDB, using replica set feature. So for local development, make sure that the orderbook app and the gateway app has the same mongoDB database.

### Migrate
to run the migration, just follow the command below

Migration up:

```bash
go run cmd/main/main.go --migrate up
```

or, for executable binary:

```bash
./main --migrate up
```

Migration down:

```bash
go run cmd/main/main.go --migrate down
```

or, for executable binary:

```bash
./main --migrate down
```

### Start the application

to start running the application, simply run

```bash
go run cmd/main/main.go
```

under cmd/main directory
this will automatically install project dependecy, just make sure you have go installed and GOPATH set in your env.

### Using docker

please setup database connection first on `.env` file by copy `.env.example` file
instead of using `localhost` or `127.0.0.1` please use `host.docker.internal` for database connection in docker

if you find connection refused error make sure that your local MySQL installation is configured to allow remote connections. by default, MySQL is configured to only accept connections from the local machine. you will need to update the bind-address setting in the MySQL configuration file (`my.cnf` or `my.ini`) to allow remote connections. to update MySQL remote connection please add or update the `bind-address` section and set it to `0.0.0.0`

to build the image you can run

```bash
docker build -t exchage-gateway .
```

and running the container by running this command

```bash
docker run --rm -p 8080:8080 exchange-gateway
```
