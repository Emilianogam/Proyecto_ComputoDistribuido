FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go build -o servicio .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/servicio .
EXPOSE 8080
CMD ["./servicio"]
