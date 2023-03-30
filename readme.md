## Options Exchange Gateway

### This projects uses Gin and implements Clean Architecture 

for gin related docs please visit  https://gin-gonic.com/docs/

for clean architecture concepts you can visit these links: 
1. https://evrone.com/go-clean-template
2. xxx

### Start the application
to start running the application, simply run 
```bash
go run main.go
```
under cmd/main directory
this will automatically install project dependecy, just make sure you have go installed and GOPATH set in your env.

###  Using docker
to build the image you can run
```bash
docker build -t exchage-gateway .
```

and running the container by running this command
```bash
docker run --rm -p 8080:8080 exchange-gateway
```
