From golang:1.18

WORKDIR /go/src/app

COPY . .

WORKDIR "cmd/main"
RUN go build -o main main.go
EXPOSE 8080
CMD ["./main"]