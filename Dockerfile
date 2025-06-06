FROM golang:1.24
LABEL authors="karel.jedlicka"

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o trd-01-backend

EXPOSE 8080

CMD ["./trd-01-backend"]
