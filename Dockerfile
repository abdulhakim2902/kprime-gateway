From golang:1.18

WORKDIR /go/src/app

COPY . .

RUN git config  --global url."https://oauth2:${ACCESS_TOKEN}@git.devucc.name".insteadOf "https://git.devucc.name"

RUN go build -o main main.go

EXPOSE 8080

CMD ["./main"]