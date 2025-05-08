FROM golang:1.23-alpine

WORKDIR /build

COPY ../.. .

RUN go mod download
RUN go build -o /bin/server .

EXPOSE 3000

CMD ["/bin/server"]