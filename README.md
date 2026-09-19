# Gestor de tareas - Sistema distribuido

Primera entrega del proyecto final de Computo Distribuido.

## Arquitectura

```
                       navegador
                           |
                    frontend (nginx)
                    localhost:3000
                           |
                    gateway (Go)
                    localhost:8000
              service discovery + round robin
                           |
        +------------------+------------------+
        |                  |                  |
   usuarios-1/2/3     tareas-1/2/3     categorias-1/2/3
        |                  |                  |
        +------------------+------------------+
                           |
                     PostgreSQL
```

Doce contenedores. Solo dos exponen puerto al exterior, el frontend y el
gateway. Los nueve servicios y la base de datos solo se alcanzan por la red
interna de Docker, usando el nombre del contenedor como hostname.

## Modulos

### Frontend
HTML, CSS y JavaScript servidos por nginx, en su propio contenedor. Solo
conoce la direccion del gateway, nunca llama a un servicio directamente.
Muestra que instancia respondio cada peticion leyendo el header
`X-Instancia`, y tiene un boton para disparar nueve peticiones seguidas y
ver el round robin en accion.

### Backend (3 servicios)
Cada servicio corre en su propio contenedor, con su propia imagen, y expone
un CRUD completo:

| Servicio   | Ruta          | Campos                    |
|------------|---------------|---------------------------|
| usuarios   | `/usuarios`   | id, nombre, email         |
| tareas     | `/tareas`     | id, titulo, completada    |
| categorias | `/categorias` | id, nombre, color         |

Cada uno corre en tres instancias para que el balanceador tenga entre que
repartir. Los tres se conectan a la misma base PostgreSQL, cada uno a su
propia tabla, que se crea sola al arrancar.

**Patron de diseno: Repository.** El archivo `store.go` de cada servicio
concentra todo el acceso a la base de datos. Los handlers HTTP de `main.go`
nunca escriben SQL: le piden los datos al repositorio. Eso separa la logica
de transporte de la de persistencia, y permitiria cambiar de base de datos
tocando un solo archivo por servicio.

### Middleware y load balancer
Un solo contenedor (`gateway`) con las dos responsabilidades:

**Service discovery.** El archivo `services.json` es el registro estatico de
los servicios y sus instancias. Ademas el gateway le pega al `/heartbeat` de
cada instancia cada diez segundos y la marca viva o caida, asi que el
registro tambien tiene una parte dinamica: si tiras un contenedor, deja de
recibir peticiones sin tener que reiniciar nada. El registro se puede
consultar en `GET /servicios`.

**Load balancing.** Round robin. El gateway lee el primer segmento de la
ruta (`/usuarios/3` -> servicio `usuarios`), escoge el pool correspondiente y
rota entre sus instancias vivas con un contador protegido por un mutex.
La peticion se reenvia con `httputil.ReverseProxy` de la libreria estandar.

## Como correrlo

Requisito unico: Docker Desktop instalado y corriendo.

```bash
./setup.sh
```

El script verifica las dependencias, construye las imagenes, levanta los
doce contenedores y hace una prueba de humo contra los tres servicios.

Si prefieres hacerlo a mano:

```bash
docker compose build
docker compose up -d
```

Luego abre <http://localhost:3000>.

## Como probarlo

```bash
# Registro de servicios con el estado de cada instancia
curl http://localhost:8000/servicios

# Round robin: fijate en el header X-Instancia, va rotando
curl -i http://localhost:8000/usuarios
curl -i http://localhost:8000/usuarios
curl -i http://localhost:8000/usuarios

# Crear
curl -X POST http://localhost:8000/usuarios \
  -H "Content-Type: application/json" \
  -d '{"nombre":"Emiliano","email":"emiliano@up.edu.mx"}'

curl -X POST http://localhost:8000/tareas \
  -H "Content-Type: application/json" \
  -d '{"titulo":"Entregar proyecto","completada":false}'

curl -X POST http://localhost:8000/categorias \
  -H "Content-Type: application/json" \
  -d '{"nombre":"Escuela","color":"azul"}'

# Tolerancia a fallos: tira una instancia y vuelve a consultar el registro
docker compose stop usuarios-2
sleep 12
curl http://localhost:8000/servicios
```

## Comandos utiles

```bash
docker compose ps              # ver los 12 contenedores
docker compose logs -f gateway # ver el ruteo en vivo
docker compose down            # apagar
docker compose down -v         # apagar y borrar la base de datos
```

## Estructura

```
proyecto-distribuido/
├── docker-compose.yml
├── setup.sh
├── README.md
├── frontend/
│   ├── index.html
│   ├── estilos.css
│   ├── app.js
│   └── Dockerfile
├── gateway/
│   ├── main.go
│   ├── services.json
│   ├── go.mod
│   └── Dockerfile
├── servicio-usuarios/
│   ├── main.go       handlers HTTP
│   ├── store.go      repositorio (acceso a datos)
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── servicio-tareas/
└── servicio-categorias/
```
