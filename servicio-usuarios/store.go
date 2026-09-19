package main

// store.go implementa el patron Repository.
// Todo el acceso a la base de datos vive aqui, los handlers nunca escriben SQL.

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Usuario struct {
	ID     int    `json:"id"`
	Nombre string `json:"nombre"`
	Email  string `json:"email"`
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
			_, err = db.Exec(`CREATE TABLE IF NOT EXISTS usuarios (
				id SERIAL PRIMARY KEY,
				nombre TEXT NOT NULL,
				email TEXT NOT NULL
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

func (s *Store) GetAll() []Usuario {
	rows, err := s.db.Query("SELECT id, nombre, email FROM usuarios ORDER BY id")
	if err != nil {
		log.Printf("error en GetAll: %v", err)
		return []Usuario{}
	}
	defer rows.Close()

	lista := []Usuario{}
	for rows.Next() {
		var u Usuario
		if err := rows.Scan(&u.ID, &u.Nombre, &u.Email); err != nil {
			log.Printf("error leyendo fila: %v", err)
			continue
		}
		lista = append(lista, u)
	}
	return lista
}

func (s *Store) Get(id int) (Usuario, bool) {
	var u Usuario
	err := s.db.QueryRow(
		"SELECT id, nombre, email FROM usuarios WHERE id = $1", id,
	).Scan(&u.ID, &u.Nombre, &u.Email)
	if err != nil {
		return Usuario{}, false
	}
	return u, true
}

func (s *Store) Create(nombre, email string) (Usuario, bool) {
	var u Usuario
	err := s.db.QueryRow(
		"INSERT INTO usuarios (nombre, email) VALUES ($1, $2) RETURNING id, nombre, email",
		nombre, email,
	).Scan(&u.ID, &u.Nombre, &u.Email)
	if err != nil {
		log.Printf("error en Create: %v", err)
		return Usuario{}, false
	}
	return u, true
}

func (s *Store) Update(id int, nombre, email string) (Usuario, bool) {
	var u Usuario
	err := s.db.QueryRow(
		"UPDATE usuarios SET nombre = $1, email = $2 WHERE id = $3 RETURNING id, nombre, email",
		nombre, email, id,
	).Scan(&u.ID, &u.Nombre, &u.Email)
	if err != nil {
		return Usuario{}, false
	}
	return u, true
}

func (s *Store) Delete(id int) bool {
	res, err := s.db.Exec("DELETE FROM usuarios WHERE id = $1", id)
	if err != nil {
		log.Printf("error en Delete: %v", err)
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}
