package main

// Servicio de auditoria: el consumidor de las colas.
//
// No tiene base de datos ni CRUD. Su unico trabajo es estar suscrito a los
// tres topicos y guardar lo que pasa por ellos. Es el otro lado de la
// mensajeria asincrona: los servicios publican sin esperar a nadie, y este
// consume cuando puede.
//
// Expone GET /eventos con los ultimos eventos recibidos, que es lo que
// muestra el frontend.

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// Un topico por servicio. Esas son las tres colas del sistema.
var topicos = []string{
	"usuarios.eventos",
	"tareas.eventos",
	"categorias.eventos",
}

type Evento struct {
	Servicio  string    `json:"servicio"`
	Accion    string    `json:"accion"`
	ID        int       `json:"id"`
	Detalle   string    `json:"detalle"`
	Instancia string    `json:"instancia"`
	Momento   time.Time `json:"momento"`

	// Estos dos no vienen en el mensaje: los agrega la auditoria para
	// poder ver de que topico y particion salio.
	Topico    string `json:"topico"`
	Particion int    `json:"particion"`
}

var (
	candado  sync.Mutex
	recibido []Evento
)

const maximoGuardado = 100

func guardar(e Evento) {
	candado.Lock()
	defer candado.Unlock()

	// Los mas nuevos primero.
	recibido = append([]Evento{e}, recibido...)
	if len(recibido) > maximoGuardado {
		recibido = recibido[:maximoGuardado]
	}
}

func listar() []Evento {
	candado.Lock()
	defer candado.Unlock()

	copia := make([]Evento, len(recibido))
	copy(copia, recibido)
	return copia
}

// crearTopicos da de alta las tres colas con 3 particiones cada una.
// Tres particiones porque hay tres instancias de cada servicio publicando.
func crearTopicos(brokers string) {
	for intento := 1; intento <= 20; intento++ {
		conexion, err := kafka.Dial("tcp", brokers)
		if err != nil {
			log.Printf("esperando a kafka (intento %d): %v", intento, err)
			time.Sleep(3 * time.Second)
			continue
		}

		controlador, err := conexion.Controller()
		conexion.Close()
		if err != nil {
			log.Printf("kafka sin controlador todavia (intento %d)", intento)
			time.Sleep(3 * time.Second)
			continue
		}

		dirControlador := net.JoinHostPort(controlador.Host, strconv.Itoa(controlador.Port))
		conexionControlador, err := kafka.Dial("tcp", dirControlador)
		if err != nil {
			log.Printf("no se pudo hablar con el controlador (intento %d): %v", intento, err)
			time.Sleep(3 * time.Second)
			continue
		}

		configuraciones := make([]kafka.TopicConfig, 0, len(topicos))
		for _, t := range topicos {
			configuraciones = append(configuraciones, kafka.TopicConfig{
				Topic:             t,
				NumPartitions:     3,
				ReplicationFactor: 1,
			})
		}

		err = conexionControlador.CreateTopics(configuraciones...)
		conexionControlador.Close()
		if err != nil {
			log.Printf("error creando topicos: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Printf("topicos listos: %v", topicos)
		return
	}

	log.Printf("no se pudieron crear los topicos, se intentara consumir de todos modos")
}

// consumir se queda escuchando los tres topicos para siempre.
// El GroupID hace que Kafka lleve el control de hasta donde leimos, asi
// que si la auditoria se reinicia, retoma donde se quedo.
func consumir(brokers string) {
	lector := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{brokers},
		GroupID:     "auditoria",
		GroupTopics: topicos,
		MinBytes:    1,
		MaxBytes:    1e6,
	})
	defer lector.Close()

	log.Printf("consumidor escuchando: %v", topicos)

	for {
		mensaje, err := lector.ReadMessage(context.Background())
		if err != nil {
			log.Printf("error leyendo de kafka: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		var evento Evento
		if err := json.Unmarshal(mensaje.Value, &evento); err != nil {
			log.Printf("mensaje ilegible en %s: %v", mensaje.Topic, err)
			continue
		}

		evento.Topico = mensaje.Topic
		evento.Particion = mensaje.Partition
		guardar(evento)

		log.Printf("consumido de %s[%d]: %s %s id=%d (publico %s)",
			mensaje.Topic, mensaje.Partition, evento.Servicio, evento.Accion,
			evento.ID, evento.Instancia)
	}
}

func eventosHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listar())
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"servicio": "auditoria",
		"estado":   "vivo",
	})
}

func main() {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "kafka:9092"
	}

	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}

	crearTopicos(brokers)
	go consumir(brokers)

	mux := http.NewServeMux()
	mux.HandleFunc("/eventos", eventosHandler)
	mux.HandleFunc("/heartbeat", heartbeatHandler)

	log.Printf("servicio auditoria escuchando en :%s", puerto)
	log.Fatal(http.ListenAndServe(":"+puerto, mux))
}
