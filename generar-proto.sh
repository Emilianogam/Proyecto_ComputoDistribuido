#!/bin/bash
# Genera el codigo Go a partir de los archivos .proto.
#
# El codigo generado ya esta versionado en el repo, asi que este script
# solo hace falta si cambias un .proto.
#
# Requiere protoc, protoc-gen-go y protoc-gen-go-grpc. Si no los tienes
# instalados, este comando los trae dentro de un contenedor:
#
#   docker run --rm -v "$PWD":/w -w /w golang:1.23 bash -c \
#     "apt-get update -qq && apt-get install -y -qq protobuf-compiler && \
#      go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
#      go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
#      PATH=\$PATH:/root/go/bin ./generar-proto.sh"

set -e

gen() {
  local proto=$1 destino=$2 paquete=$3
  mkdir -p "$destino"
  protoc -I proto \
    --go_out="$destino"      --go_opt=paths=source_relative      --go_opt=M"$proto"="$paquete" \
    --go-grpc_out="$destino" --go-grpc_opt=paths=source_relative --go-grpc_opt=M"$proto"="$paquete" \
    "proto/$proto"
  echo "  $proto -> $destino"
}

echo "Generando codigo de protobuf y gRPC..."

# Cada servicio tiene el codigo de su propio contrato.
gen usuarios.proto   servicio-usuarios/pb/usuariospb     servicio-usuarios/pb/usuariospb
gen tareas.proto     servicio-tareas/pb/tareaspb         servicio-tareas/pb/tareaspb
gen categorias.proto servicio-categorias/pb/categoriaspb servicio-categorias/pb/categoriaspb

# tareas ademas es cliente de los otros dos, asi que necesita sus contratos.
gen usuarios.proto   servicio-tareas/pb/usuariospb       servicio-tareas/pb/usuariospb
gen categorias.proto servicio-tareas/pb/categoriaspb     servicio-tareas/pb/categoriaspb

echo "Listo."
