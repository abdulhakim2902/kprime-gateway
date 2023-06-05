<p align="center">
    <h1 align="center">K-Prime Gateway</h1>
</p>

## Requirement
Make sure you have Go installed and GOPATH set in your env.
Go version: `1.20.3` <br />
MongoDB version: Community Edition `6.0.5`


### Framework
This projects uses `Gin` and implements `Clean Architecture`

* for gin related docs please visit https://gin-gonic.com/docs/
* for clean architecture concepts you can visit the link: https://evrone.com/go-clean-template


### Setup the application

To setup the application, please copy file `.env.example` and rename it to `.env`, modify `.env` file based on your local requirement.

Add GOPRIVATE to the go env for git.devucc.name to get private dependencies
```bash
GOPRIVATE=git.devucc.name/dependencies/utilities go get git.devucc.name/dependencies/utilities@v1.0.3
```

### Database
We're using two databases:
* MySQL
* MongoDB

MySQL is used only for gateway application, to store user's credentials, roles, permissions, etc. 

Gateway is connected to the orderbook's MongoDB instance. In production Gateway would use replica set feature. For local development, make sure that the `orderbook app` and the `gateway app` has the same MongoDB database.

### Migrate
To run the migration, just follow the command below

Migration up:

```bash
go run main.go --migrate up
```

or, for executable binary:

```bash
./main --migrate up
```

Migration down:

```bash
go run main.go --migrate down
```

or, for executable binary:

```bash
./main --migrate down
```

### Start the application

To start running the application, simply run:

```bash
go run main.go
```

under root directory this will automatically install project dependecy.

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

### Metrics

Run in port `2112` (default), Metrics endpoint:

```bash
<host>:2112/metrics
```

Custom port with env variable:

`METRICS_PORT="2113"`
