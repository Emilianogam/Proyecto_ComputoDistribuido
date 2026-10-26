#!/bin/bash
# Script de automatizacion del proyecto.
# Verifica las dependencias, construye las imagenes, levanta los contenedores
# y hace una prueba rapida de que todo responde.

set -e

echo "=============================================="
echo " Gestor de tareas - sistema distribuido"
echo "=============================================="
echo ""

# --- 1. Verificar dependencias ---

echo "[1/4] Verificando dependencias..."

if ! command -v docker > /dev/null; then
  echo "ERROR: Docker no esta instalado."
  echo "Instalalo desde https://www.docker.com/products/docker-desktop"
  exit 1
fi

if ! docker info > /dev/null 2>&1; then
  echo "ERROR: Docker esta instalado pero el demonio no esta corriendo."
  echo "Abre Docker Desktop y vuelve a correr este script."
  exit 1
fi

if ! docker compose version > /dev/null 2>&1; then
  echo "ERROR: docker compose no esta disponible."
  exit 1
fi

echo "      Docker OK: $(docker --version)"
echo "      Compose OK: $(docker compose version --short)"
echo ""

# --- 2. Construir las imagenes ---
# Cada servicio baja sus propias dependencias de Go dentro de su
# contenedor, asi que no hace falta tener Go instalado en la maquina.

echo "[2/4] Construyendo las imagenes (la primera vez tarda varios minutos)..."
docker compose build
echo ""

# --- 3. Levantar todo ---

echo "[3/4] Levantando los contenedores..."
docker compose up -d
echo ""

echo "      Esperando a que los servicios se registren en Consul..."
sleep 20

# --- 4. Prueba rapida ---

echo "[4/4] Probando el sistema..."
echo ""

codigo=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8500/v1/status/leader" || echo "000")
if [ "$codigo" = "200" ]; then
  echo "      consul                OK ($codigo)"
else
  echo "      consul                FALLO ($codigo)"
fi

for servicio in usuarios tareas categorias; do
  registradas=$(curl -s "http://localhost:8000/consul/v1/health/service/$servicio" | grep -o '"ServiceID"' | wc -l | tr -d ' ')
  codigo=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8000/$servicio" || echo "000")
  if [ "$codigo" = "200" ]; then
    echo "      /$servicio            OK ($codigo) - $registradas instancias en consul"
  else
    echo "      /$servicio            FALLO ($codigo)"
  fi
done

echo ""
echo "      Balanceo de nginx sobre /usuarios:"
for i in 1 2 3 4 5 6; do
  instancia=$(curl -s -D - -o /dev/null "http://localhost:8000/usuarios" | grep -i "^X-Instancia" | tr -d '\r' | awk '{print $2}')
  echo "        peticion $i -> $instancia"
done

echo ""
echo "=============================================="
echo " Listo."
echo ""
echo " Frontend:            http://localhost:3000"
echo " nginx (gateway):     http://localhost:8000"
echo " Consul (interfaz):   http://localhost:8500"
echo ""
echo " gRPC usuarios:       localhost:9001"
echo " gRPC tareas:         localhost:9002"
echo " gRPC categorias:     localhost:9003"
echo " Probar gRPC:         grpcurl -plaintext localhost:9001 list"
echo ""
echo " Eventos de Kafka:    http://localhost:8000/eventos"
echo " Ver los topicos:     docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --list"
echo " Ver el consumo:      docker compose logs -f auditoria"
echo " Registro via nginx:  http://localhost:8000/consul/v1/health/service/usuarios"
echo ""
echo " Ver contenedores:    docker compose ps"
echo " Ver el balanceo:     docker compose logs -f nginx"
echo " Apagar todo:         docker compose down"
echo "=============================================="
