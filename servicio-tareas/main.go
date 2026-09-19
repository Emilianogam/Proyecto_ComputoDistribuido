package main

// Servicio de tareas. Expone un CRUD sobre /tareas y un /heartbeat
// que el gateway usa para saber si esta instancia sigue viva.

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var store *Store

// instanciaID identifica al contenedor que respondio.
// Lo mandamos en un header para poder ver el balanceo en la demo.
var instanciaID = "desconocida"

func conHeaders(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Instancia", instanciaID)
		siguiente(w, r)
	}
}

// /tareas -> GET (lista) y POST (crear)
func tareasHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		json.NewEncoder(w).Encode(store.GetAll())

	case http.MethodPost:
		var body struct {
			Titulo     string `json:"titulo"`
			Completada bool   `json:"completada"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "json invalido", http.StatusBadRequest)
			return
		}
		if body.Titulo == "" {
			http.Error(w, "titulo requerido", http.StatusBadRequest)
			return
		}
		t, ok := store.Create(body.Titulo, body.Completada)
		if !ok {
			http.Error(w, "no se pudo crear", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(t)

	default:
		http.Error(w, "metodo no permitido", http.StatusMethodNotAllowed)
	}
}

// /tareas/{id} -> GET, PUT, DELETE
func tareaHandler(w http.ResponseWriter, r *http.Request) {
	idTexto := strings.TrimPrefix(r.URL.Path, "/tareas/")
	id, err := strconv.Atoi(idTexto)
	if err != nil {
		http.Error(w, "id invalido", http.StatusBadRequest)
		return
	}

	switch r.Method {

	case http.MethodGet:
		t, ok := store.Get(id)
		if !ok {
			http.Error(w, "no encontrado", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(t)

	case http.MethodPut:
		var body struct {
			Titulo     string `json:"titulo"`
			Completada bool   `json:"completada"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "json invalido", http.StatusBadRequest)
			return
		}
		t, ok := store.Update(id, body.Titulo, body.Completada)
		if !ok {
			http.Error(w, "no encontrado", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(t)

	case http.MethodDelete:
		if !store.Delete(id) {
			http.Error(w, "no encontrado", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "metodo no permitido", http.StatusMethodNotAllowed)
	}
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"servicio":  "tareas",
		"instancia": instanciaID,
		"estado":    "vivo",
	})
}

func main() {
	if v := os.Getenv("INSTANCIA_ID"); v != "" {
		instanciaID = v
	}

	store = NuevoStore(os.Getenv("DATABASE_URL"))

	// Dentro de Docker todos escuchan en 8080. PORT existe para
	// poder levantar varias instancias fuera de Docker al probar.
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/tareas", conHeaders(tareasHandler))
	mux.HandleFunc("/tareas/", conHeaders(tareaHandler))
	mux.HandleFunc("/heartbeat", conHeaders(heartbeatHandler))

	log.Printf("servicio tareas [%s] escuchando en :%s", instanciaID, puerto)
	log.Fatal(http.ListenAndServe(":"+puerto, mux))
}
