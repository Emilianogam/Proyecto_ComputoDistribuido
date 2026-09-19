package main

// store.go implementa el patron Repository.
// Todo el acceso a la base de datos vive aqui, los handlers nunca escriben SQL.

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Tarea struct {
	ID         int    `json:"id"`
	Titulo     string `json:"titulo"`
	Completada bool   `json:"completada"`
}

type Store struct {
	db *sql.DB
}

// NuevoStore abre la conexion y reintenta, porque Postgres tarda unos
// segundos en aceptar conexiones cuando arranca el docker compose.
func NuevoStore(dsn string) *Store {
	var db *sql.DB
	var err error

	for i := 0; i < 20; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tareas (
				id SERIAL PRIMARY KEY,
				titulo TEXT NOT NULL,
				completada BOOLEAN NOT NULL DEFAULT FALSE
			)`)
		}
		if err == nil {
			log.Println("conectado a postgres")
			return &Store{db: db}
		}
		log.Printf("esperando a postgres (intento %d): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	log.Fatalf("no se pudo conectar a postgres: %v", err)
	return nil
}

func (s *Store) GetAll() []Tarea {
	rows, err := s.db.Query("SELECT id, titulo, completada FROM tareas ORDER BY id")
	if err != nil {
		log.Printf("error en GetAll: %v", err)
		return []Tarea{}
	}
	defer rows.Close()

	lista := []Tarea{}
	for rows.Next() {
		var t Tarea
		if err := rows.Scan(&t.ID, &t.Titulo, &t.Completada); err != nil {
			log.Printf("error leyendo fila: %v", err)
			continue
		}
		lista = append(lista, t)
	}
	return lista
}

func (s *Store) Get(id int) (Tarea, bool) {
	var t Tarea
	err := s.db.QueryRow(
		"SELECT id, titulo, completada FROM tareas WHERE id = $1", id,
	).Scan(&t.ID, &t.Titulo, &t.Completada)
	if err != nil {
		return Tarea{}, false
	}
	return t, true
}

func (s *Store) Create(titulo string, completada bool) (Tarea, bool) {
	var t Tarea
	err := s.db.QueryRow(
		"INSERT INTO tareas (titulo, completada) VALUES ($1, $2) RETURNING id, titulo, completada",
		titulo, completada,
	).Scan(&t.ID, &t.Titulo, &t.Completada)
	if err != nil {
		log.Printf("error en Create: %v", err)
		return Tarea{}, false
	}
	return t, true
}

func (s *Store) Update(id int, titulo string, completada bool) (Tarea, bool) {
	var t Tarea
	err := s.db.QueryRow(
		"UPDATE tareas SET titulo = $1, completada = $2 WHERE id = $3 RETURNING id, titulo, completada",
		titulo, completada, id,
	).Scan(&t.ID, &t.Titulo, &t.Completada)
	if err != nil {
		return Tarea{}, false
	}
	return t, true
}

func (s *Store) Delete(id int) bool {
	res, err := s.db.Exec("DELETE FROM tareas WHERE id = $1", id)
	if err != nil {
		log.Printf("error en Delete: %v", err)
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}
