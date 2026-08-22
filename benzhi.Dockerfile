FROM golang:1.26.5

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build ./...

CMD ["bash"]
