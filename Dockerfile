FROM golang:1.18
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY ./ ./
RUN go build -o server main.go
ENTRYPOINT ["./server"]
