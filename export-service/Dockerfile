FROM golang:1.20-alpine

RUN apk add --no-cache git

WORKDIR /export-service

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go install github.com/codegangsta/gin@latest

EXPOSE 8080

CMD ["gin", "--port", "8080","run", "main.go"]