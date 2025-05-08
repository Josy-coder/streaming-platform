FROM golang:1.23-alpine

RUN apk add --no-cache gcc musl-dev

ENV CGO_ENABLED=1

WORKDIR /build

COPY . .

RUN go mod download
RUN go build -o /bin/server .

EXPOSE 3000

CMD ["/bin/server"]