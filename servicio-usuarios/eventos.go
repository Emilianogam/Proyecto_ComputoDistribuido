package main

// eventos.go es el productor de Kafka.
//
// Cada vez que el servicio crea, actualiza o borra un registro, publica un
// evento en su propio topico. Es mensajeria ASINCRONA: la peticion HTTP del
// usuario ya respondio cuando el mensaje sale, y a nadie le importa si hay
// alguien escuchando o no. Si Kafka esta caido, el CRUD sigue funcionando.
//
// Cada servicio tiene su propio topico, su propia cola:
//   usuarios.eventos   tareas.eventos   categorias.eventos

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

// Evento es el mensaje que viaja por la cola.
type Evento struct {
	Servicio  string    `json:"servicio"`
	Accion    string    `json:"accion"`
	ID        int       `json:"id"`
	Detalle   string    `json:"detalle"`
	Instancia string    `json:"instancia"`
	Momento   time.Time `json:"momento"`
}

var (
	escritor *kafka.Writer
	topico   string
	servicio string
)

// arrancarKafka deja listo el productor. No bloquea: si Kafka todavia no
// esta arriba, los envios fallaran y se reintentaran despues.
func arrancarKafka(nombreServicio string) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "kafka:9092"
	}

	servicio = nombreServicio
	topico = nombreServicio + ".eventos"

	escritor = &kafka.Writer{
		Addr:  kafka.TCP(brokers),
		Topic: topico,
		// Hash manda los mensajes con la misma llave a la misma particion,
		// asi los eventos de un mismo registro conservan su orden.
		Balancer:               &kafka.Hash{},
		AllowAutoTopicCreation: true,
		BatchTimeout:           200 * time.Millisecond,
	}

	log.Printf("productor kafka listo: broker %s, topico %s", brokers, topico)
}

// publicar manda un evento a la cola sin bloquear al que lo llama.
// Esa es la parte asincrona: el handler HTTP ya contesto.
func publicar(accion string, id int, detalle string) {
	if escritor == nil {
		return
	}

	evento := Evento{
		Servicio:  servicio,
		Accion:    accion,
		ID:        id,
		Detalle:   detalle,
		Instancia: instanciaID,
		Momento:   time.Now().UTC(),
	}

	cuerpo, err := json.Marshal(evento)
	if err != nil {
		log.Printf("no se pudo armar el evento: %v", err)
		return
	}

	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelar()

		err := escritor.WriteMessages(ctx, kafka.Message{
			Key:   []byte(strconv.Itoa(id)),
			Value: cuerpo,
		})
		if err != nil {
			log.Printf("no se pudo publicar en %s: %v", topico, err)
			return
		}
		log.Printf("evento publicado en %s: %s id=%d", topico, accion, id)
	}()
}
