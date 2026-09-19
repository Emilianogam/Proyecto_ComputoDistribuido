FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN go build -o gateway .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/gateway .
COPY services.json .
EXPOSE 8000
CMD ["./gateway"]
