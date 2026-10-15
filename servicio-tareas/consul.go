package main

// consul.go implementa el service discovery dinamico.
//
// Al arrancar, la instancia se registra sola en Consul: le dice su nombre,
// su direccion y donde esta su health check. Consul se encarga de llamar a
// /heartbeat cada 10 segundos y marcarla viva o caida.
//
// Al apagarse, la instancia se borra sola del registro.
//
// La diferencia con la version anterior: antes el registro era un archivo
// estatico que habia que editar a mano. Ahora las instancias aparecen y
// desaparecen solas.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type consulCheck struct {
	HTTP                           string `json:"HTTP"`
	Interval                       string `json:"Interval"`
	Timeout                        string `json:"Timeout"`
	DeregisterCriticalServiceAfter string `json:"DeregisterCriticalServiceAfter"`
}

type consulServicio struct {
	ID      string      `json:"ID"`
	Name    string      `json:"Name"`
	Address string      `json:"Address"`
	Port    int         `json:"Port"`
	Check   consulCheck `json:"Check"`
}

var consulURL = "http://consul:8500"

// registrarEnConsul da de alta esta instancia en el registro de Consul.
func registrarEnConsul(nombreServicio, instanciaID, puerto string) {
	if v := os.Getenv("CONSUL_URL"); v != "" {
		consulURL = v
	}

	numPuerto, _ := strconv.Atoi(puerto)

	servicio := consulServicio{
		ID:      instanciaID,
		Name:    nombreServicio,
		Address: instanciaID, // el nombre del contenedor es el hostname en la red de Docker
		Port:    numPuerto,
		Check: consulCheck{
			HTTP:                           fmt.Sprintf("http://%s:%s/heartbeat", instanciaID, puerto),
			Interval:                       "10s",
			Timeout:                        "2s",
			DeregisterCriticalServiceAfter: "1m",
		},
	}

	cuerpo, err := json.Marshal(servicio)
	if err != nil {
		log.Printf("error armando el registro de consul: %v", err)
		return
	}

	cliente := &http.Client{Timeout: 3 * time.Second}

	// Consul puede tardar en arrancar, igual que Postgres. Reintentamos.
	for i := 0; i < 15; i++ {
		req, _ := http.NewRequest("PUT", consulURL+"/v1/agent/service/register", bytes.NewReader(cuerpo))
		req.Header.Set("Content-Type", "application/json")

		resp, err := cliente.Do(req)
		if err == nil {
			codigo := resp.StatusCode
			resp.Body.Close()
			if codigo == http.StatusOK {
				log.Printf("registrado en consul: %s (servicio %s)", instanciaID, nombreServicio)
				return
			}
		}
		log.Printf("esperando a consul (intento %d)", i+1)
		time.Sleep(2 * time.Second)
	}

	log.Printf("no se pudo registrar en consul, el servicio sigue atendiendo peticiones")
}

// borrarDeConsul quita esta instancia del registro.
func borrarDeConsul(instanciaID string) {
	cliente := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("PUT", consulURL+"/v1/agent/service/deregister/"+instanciaID, nil)

	resp, err := cliente.Do(req)
	if err != nil {
		log.Printf("error borrando de consul: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("borrado de consul: %s", instanciaID)
}

// esperarApagado escucha la senal que Docker manda al detener el contenedor
// y borra la instancia del registro antes de salir. Esto es el "borrado del
// service registry" que pide la entrega.
func esperarApagado(instanciaID string) {
	canal := make(chan os.Signal, 1)
	signal.Notify(canal, syscall.SIGTERM, syscall.SIGINT)

	<-canal
	log.Printf("apagando %s, borrando del registro", instanciaID)
	borrarDeConsul(instanciaID)
	os.Exit(0)
}
