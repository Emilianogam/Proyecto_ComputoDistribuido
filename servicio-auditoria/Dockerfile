FROM golang:1.23-alpine AS build
WORKDIR /app
COPY . .
# go mod tidy resuelve las dependencias de gRPC y completa go.sum dentro
# del contenedor, asi no hace falta tener Go instalado en la maquina.
RUN go mod tidy
RUN go build -o servicio .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/servicio .
EXPOSE 8080
CMD ["./servicio"]
