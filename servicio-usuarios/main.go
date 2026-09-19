package main

// Servicio de usuarios. Expone un CRUD sobre /usuarios y un /heartbeat
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

// /usuarios  -> GET (lista) y POST (crear)
func usuariosHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		json.NewEncoder(w).Encode(store.GetAll())

	case http.MethodPost:
		var body struct {
			Nombre string `json:"nombre"`
			Email  string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "json invalido", http.StatusBadRequest)
			return
		}
		if body.Nombre == "" {
			http.Error(w, "nombre requerido", http.StatusBadRequest)
			return
		}
		u, ok := store.Create(body.Nombre, body.Email)
		if !ok {
			http.Error(w, "no se pudo crear", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(u)

	default:
		http.Error(w, "metodo no permitido", http.StatusMethodNotAllowed)
	}
}

// /usuarios/{id} -> GET, PUT, DELETE
func usuarioHandler(w http.ResponseWriter, r *http.Request) {
	idTexto := strings.TrimPrefix(r.URL.Path, "/usuarios/")
	id, err := strconv.Atoi(idTexto)
	if err != nil {
		http.Error(w, "id invalido", http.StatusBadRequest)
		return
	}

	switch r.Method {

	case http.MethodGet:
		u, ok := store.Get(id)
		if !ok {
			http.Error(w, "no encontrado", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(u)

	case http.MethodPut:
		var body struct {
			Nombre string `json:"nombre"`
			Email  string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "json invalido", http.StatusBadRequest)
			return
		}
		u, ok := store.Update(id, body.Nombre, body.Email)
		if !ok {
			http.Error(w, "no encontrado", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(u)

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
		"servicio":  "usuarios",
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
	mux.HandleFunc("/usuarios", conHeaders(usuariosHandler))
	mux.HandleFunc("/usuarios/", conHeaders(usuarioHandler))
	mux.HandleFunc("/heartbeat", conHeaders(heartbeatHandler))

	log.Printf("servicio usuarios [%s] escuchando en :%s", instanciaID, puerto)
	log.Fatal(http.ListenAndServe(":"+puerto, mux))
}
