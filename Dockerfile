FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o server main.go

EXPOSE 8080

CMD ["./server"]
