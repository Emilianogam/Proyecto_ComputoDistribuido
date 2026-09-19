package main

// Middleware del sistema. Hace dos cosas:
//
//   1. Service discovery: lee services.json, que es el registro de los
//      servicios y sus instancias. Ademas revisa el /heartbeat de cada
//      instancia cada 10 segundos para marcarla viva o caida.
//
//   2. Load balancing: reparte las peticiones entre las instancias vivas
//      de cada servicio usando round robin.
//
// El frontend solo conoce este gateway, nunca habla con un servicio directo.

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Instancia es un contenedor concreto de un servicio.
type Instancia struct {
	URL  string `json:"url"`
	Viva bool   `json:"viva"`
}

// Pool son todas las instancias de un mismo servicio.
type Pool struct {
	Nombre      string
	Instancias  []*Instancia
	siguienteID int
	mu          sync.Mutex
}

// Siguiente aplica round robin: avanza el contador y regresa la
// siguiente instancia viva. Si ninguna esta viva, usa la que toque
// de todos modos para no dejar de responder.
func (p *Pool) Siguiente() *Instancia {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := len(p.Instancias)
	for i := 0; i < total; i++ {
		inst := p.Instancias[p.siguienteID%total]
		p.siguienteID++
		if inst.Viva {
			return inst
		}
	}

	inst := p.Instancias[p.siguienteID%total]
	p.siguienteID++
	return inst
}

// registro guarda los pools por nombre de servicio: "usuarios", "tareas", ...
var registro = map[string]*Pool{}

// cargarRegistro lee services.json y arma los pools.
func cargarRegistro(ruta string) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		log.Fatalf("no se pudo leer %s: %v", ruta, err)
	}

	var crudo map[string][]string
	if err := json.Unmarshal(datos, &crudo); err != nil {
		log.Fatalf("services.json invalido: %v", err)
	}

	for servicio, urls := range crudo {
		pool := &Pool{Nombre: servicio}
		for _, u := range urls {
			pool.Instancias = append(pool.Instancias, &Instancia{URL: u, Viva: true})
		}
		registro[servicio] = pool
		log.Printf("servicio registrado: %s con %d instancias", servicio, len(urls))
	}
}

// revisarSalud le pega al /heartbeat de cada instancia cada 10 segundos.
// Esto es lo que hace el discovery un poco dinamico: si tiras un contenedor,
// el gateway deja de mandarle peticiones.
func revisarSalud() {
	cliente := &http.Client{Timeout: 2 * time.Second}

	for {
		for _, pool := range registro {
			for _, inst := range pool.Instancias {
				resp, err := cliente.Get(inst.URL + "/heartbeat")
				viva := err == nil && resp.StatusCode == http.StatusOK
				if resp != nil {
					resp.Body.Close()
				}
				if viva != inst.Viva {
					log.Printf("%s: %s ahora esta %v", pool.Nombre, inst.URL, estado(viva))
				}
				inst.Viva = viva
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func estado(viva bool) string {
	if viva {
		return "viva"
	}
	return "caida"
}

// cors deja que el frontend (puerto 3000) llame al gateway (puerto 8000).
func cors(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "X-Instancia, X-Gateway-Destino")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		siguiente(w, r)
	}
}

// enrutar toma el primer segmento de la ruta para saber a que servicio va.
// /usuarios/3 -> servicio "usuarios"
func enrutar(w http.ResponseWriter, r *http.Request) {
	partes := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	servicio := partes[0]

	pool, existe := registro[servicio]
	if !existe {
		http.Error(w, "servicio no registrado: "+servicio, http.StatusNotFound)
		return
	}

	inst := pool.Siguiente()

	destino, err := url.Parse(inst.URL)
	if err != nil {
		http.Error(w, "url de instancia invalida", http.StatusInternalServerError)
		return
	}

	log.Printf("%s %s -> %s", r.Method, r.URL.Path, inst.URL)
	w.Header().Set("X-Gateway-Destino", inst.URL)

	proxy := httputil.NewSingleHostReverseProxy(destino)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("error hablando con %s: %v", inst.URL, err)
		inst.Viva = false
		http.Error(w, "la instancia no respondio", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

// serviciosHandler expone el registro para poder verlo en el frontend.
func serviciosHandler(w http.ResponseWriter, r *http.Request) {
	salida := map[string][]*Instancia{}
	for nombre, pool := range registro {
		salida[nombre] = pool.Instancias
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(salida)
}

func main() {
	ruta := os.Getenv("SERVICES_FILE")
	if ruta == "" {
		ruta = "services.json"
	}
	cargarRegistro(ruta)

	go revisarSalud()

	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8000"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/servicios", cors(serviciosHandler))
	mux.HandleFunc("/", cors(enrutar))

	log.Printf("gateway escuchando en :%s", puerto)
	log.Fatal(http.ListenAndServe(":"+puerto, mux))
}
