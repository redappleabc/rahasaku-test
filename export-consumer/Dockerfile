FROM golang:1.20-alpine

RUN apk add --no-cache git

WORKDIR /export-consumer

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go install github.com/codegangsta/gin@latest
RUN go install github.com/pilu/fresh
RUN go install github.com/cosmtrek/air

EXPOSE 8080

CMD ["gin", "--port", "8080","run", "main.go"]