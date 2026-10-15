# Gestor de tareas - Sistema distribuido

Proyecto final de Computo Distribuido.

## Entregas

| Entrega | Contenido |
|---|---|
| 1 | Frontend, 3 servicios con 3 instancias cada uno, base de datos, middleware propio en Go con service discovery estatico y round robin |
| 2 | Service discovery dinamico con **Consul**, y **nginx** como reverse proxy, gateway y load balancer |

En la entrega 2 el middleware escrito en Go se reemplazo por nginx, y el
archivo estatico `services.json` se reemplazo por el registro de Consul.

## Arquitectura actual

```
                       navegador
                           |
                    frontend (nginx)
                    localhost:3000
                           |
                  nginx (reverse proxy,
                  gateway y load balancer)
                    localhost:8000
                           |
        +------------------+------------------+
        |                  |                  |
   usuarios-1/2/3     tareas-1/2/3     categorias-1/2/3
        |                  |                  |
        +------------------+------------------+
                           |
                     PostgreSQL

   Consul (localhost:8500) registra las 9 instancias
   y les hace health check cada 10 segundos.
```

Catorce contenedores. Solo tres exponen puerto al exterior: el frontend
(3000), nginx (8000) y la interfaz de Consul (8500). Los nueve servicios y la
base de datos solo se alcanzan por la red interna de Docker, usando el nombre
del contenedor como hostname.

## Modulos

### Frontend
HTML, CSS y JavaScript servidos por nginx, en su propio contenedor. Solo
conoce la direccion del proxy, nunca llama a un servicio directamente.
Muestra que instancia respondio cada peticion leyendo el header
`X-Instancia`, y tiene un boton que dispara nueve peticiones seguidas para
ver el round robin en accion. El panel de registro de servicios se construye
consultando la API de Consul a traves de nginx.

### Backend (3 servicios)
Cada servicio corre en su propio contenedor, con su propia imagen, y expone
un CRUD completo:

| Servicio   | Ruta          | Campos                    |
|------------|---------------|---------------------------|
| usuarios   | `/usuarios`   | id, nombre, email         |
| tareas     | `/tareas`     | id, titulo, completada    |
| categorias | `/categorias` | id, nombre, color         |

Cada uno corre en tres instancias. Los tres se conectan a la misma base
PostgreSQL, cada uno a su propia tabla, que se crea sola al arrancar.

**Patron de diseno: Repository.** El archivo `store.go` de cada servicio
concentra todo el acceso a la base de datos. Los handlers HTTP de `main.go`
nunca escriben SQL: le piden los datos al repositorio.

Los servicios son stateless: no guardan nada en memoria entre peticiones.
Todo el estado vive en PostgreSQL, y por eso las tres instancias de un
servicio son intercambiables.

### Service discovery: Consul

Consul corre en su propio contenedor y es el registro de servicios.

**Registro.** Al arrancar, cada instancia se da de alta sola llamando a la
API de Consul (`PUT /v1/agent/service/register`). Manda su ID, el nombre del
servicio al que pertenece, su direccion y su puerto. El codigo esta en
`consul.go` de cada servicio, en la funcion `registrarEnConsul`.

**Health check.** En el mismo registro, la instancia le dice a Consul donde
esta su endpoint de salud. Consul llama a `http://<instancia>:8080/heartbeat`
cada 10 segundos con un timeout de 2 segundos, y marca la instancia como
`passing` o `critical`. Si una instancia lleva mas de un minuto en estado
critico, Consul la borra del registro sola.

**Borrado.** Al detener el contenedor, Docker manda SIGTERM. La instancia
atrapa esa senal, llama a `PUT /v1/agent/service/deregister/<id>` y despues
sale. El codigo esta en las funciones `esperarApagado` y `borrarDeConsul`.

La diferencia con la entrega anterior: antes el registro era un archivo
estatico que habia que editar a mano y reiniciar el middleware. Ahora las
instancias aparecen y desaparecen del registro solas.

El registro se puede ver en la interfaz de Consul en <http://localhost:8500>,
o en JSON en `http://localhost:8000/consul/v1/health/service/usuarios`.

### Reverse proxy, gateway y load balancer: nginx

nginx corre en su propio contenedor y es la unica puerta de entrada al
backend. Toda la configuracion esta en `nginx/nginx.conf`.

**Reverse proxy.** Recibe la peticion del navegador y la reenvia al backend
con `proxy_pass`. El cliente nunca sabe con que contenedor hablo.

**Gateway / enrutamiento.** El primer segmento de la ruta decide el servicio.
`/usuarios/3` entra al bloque `location /usuarios` y se va al upstream de
usuarios. Ademas expone `/consul/` para que el frontend pueda consultar el
registro sin hablar con Consul directamente.

**Load balancing.** Cada servicio tiene un bloque `upstream` con sus tres
instancias. nginx reparte entre ellas con round robin, que es su algoritmo
por defecto. Con `max_fails=1` y `fail_timeout=10s`, si una instancia falla
una peticion nginx deja de mandarle trafico durante diez segundos.

### Script de automatizacion
`setup.sh` verifica las dependencias, construye las imagenes, levanta los
catorce contenedores y hace una prueba de humo contra Consul y los tres
servicios.

## Como correrlo

Requisito unico: Docker Desktop instalado y corriendo.

```bash
./setup.sh
```

Si prefieres hacerlo a mano:

```bash
docker compose build
docker compose up -d
```

Luego abre <http://localhost:3000>.

## Como probarlo

```bash
# Registro de servicios en Consul, con el estado de cada instancia
curl http://localhost:8000/consul/v1/health/service/usuarios

# Round robin de nginx: fijate en el header X-Instancia, va rotando
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

# Borrado del registro: al detener la instancia se borra sola de Consul
docker compose stop usuarios-2
curl http://localhost:8000/consul/v1/health/service/usuarios

# Y al levantarla se vuelve a registrar sola
docker compose start usuarios-2
sleep 5
curl http://localhost:8000/consul/v1/health/service/usuarios
```

## Comandos utiles

```bash
docker compose ps              # ver los contenedores
docker compose logs -f nginx   # ver el balanceo en vivo
docker compose logs -f consul  # ver los registros y bajas
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
├── nginx/
│   ├── nginx.conf     upstreams, round robin y enrutamiento
│   └── Dockerfile
├── servicio-usuarios/
│   ├── main.go        handlers HTTP y heartbeat
│   ├── store.go       repositorio (acceso a datos)
│   ├── consul.go      registro, health check y borrado en Consul
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── servicio-tareas/
└── servicio-categorias/
```

## Pendientes para la entrega final

Los upstreams de nginx estan escritos a mano en `nginx.conf`. El siguiente
paso natural es generarlos desde el registro de Consul con `consul-template`,
para que nginx tambien se entere solo cuando aparece o desaparece una
instancia.

La base de datos es un punto unico de falla: los tres servicios dependen de
una sola instancia de PostgreSQL.
