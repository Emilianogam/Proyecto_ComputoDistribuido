package main

// clientes.go es la comunicacion entre servicios.
//
// Cuando tareas necesita datos de usuarios o de categorias, no consulta su
// base ni les pega por HTTP: los llama por gRPC usando el contrato de cada
// uno.
//
// Importante: las llamadas NO van directo a una instancia. Van al load
// balancer (nginx), que escoge una de las tres por round robin. Por eso las
// direcciones apuntan a nginx y no a usuarios-1 o categorias-2.

import (
	"context"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	catpb "servicio-tareas/pb/categoriaspb"
	usrpb "servicio-tareas/pb/usuariospb"
)

func direccion(variable, porDefecto string) string {
	if v := os.Getenv(variable); v != "" {
		return v
	}
	return porDefecto
}

// conectar abre una conexion gRPC sin TLS, que es lo correcto aqui porque
// el trafico nunca sale de la red interna de Docker.
func conectar(destino string) (*grpc.ClientConn, error) {
	return grpc.NewClient(destino, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// contarUsuarios llama al RPC Contar del servicio de usuarios.
// Regresa el total y que instancia contesto.
func contarUsuarios(ctx context.Context) (int32, string) {
	destino := direccion("USUARIOS_GRPC", "nginx:9001")

	conexion, err := conectar(destino)
	if err != nil {
		log.Printf("no se pudo conectar a usuarios (%s): %v", destino, err)
		return 0, "sin respuesta"
	}
	defer conexion.Close()

	ctx, cancelar := context.WithTimeout(ctx, 3*time.Second)
	defer cancelar()

	resp, err := usrpb.NewUsuariosClient(conexion).Contar(ctx, &usrpb.ContarRequest{})
	if err != nil {
		log.Printf("error llamando a usuarios.Contar: %v", err)
		return 0, "sin respuesta"
	}

	log.Printf("grpc usuarios.Contar -> %d (instancia %s)", resp.GetTotal(), resp.GetInstancia())
	return resp.GetTotal(), resp.GetInstancia()
}

// contarCategorias llama al RPC Contar del servicio de categorias.
func contarCategorias(ctx context.Context) (int32, string) {
	destino := direccion("CATEGORIAS_GRPC", "nginx:9003")

	conexion, err := conectar(destino)
	if err != nil {
		log.Printf("no se pudo conectar a categorias (%s): %v", destino, err)
		return 0, "sin respuesta"
	}
	defer conexion.Close()

	ctx, cancelar := context.WithTimeout(ctx, 3*time.Second)
	defer cancelar()

	resp, err := catpb.NewCategoriasClient(conexion).Contar(ctx, &catpb.ContarRequest{})
	if err != nil {
		log.Printf("error llamando a categorias.Contar: %v", err)
		return 0, "sin respuesta"
	}

	log.Printf("grpc categorias.Contar -> %d (instancia %s)", resp.GetTotal(), resp.GetInstancia())
	return resp.GetTotal(), resp.GetInstancia()
}

// resumen junta los datos de los tres servicios. Es la unica operacion del
// sistema que necesita hablar con mas de un servicio.
type datosResumen struct {
	Usuarios            int32  `json:"usuarios"`
	Tareas              int32  `json:"tareas"`
	Categorias          int32  `json:"categorias"`
	Instancia           string `json:"instancia"`
	InstanciaUsuarios   string `json:"instancia_usuarios"`
	InstanciaCategorias string `json:"instancia_categorias"`
}

func armarResumen(ctx context.Context) datosResumen {
	totalUsuarios, instanciaUsuarios := contarUsuarios(ctx)
	totalCategorias, instanciaCategorias := contarCategorias(ctx)

	return datosResumen{
		Usuarios:            totalUsuarios,
		Tareas:              int32(len(store.GetAll())),
		Categorias:          totalCategorias,
		Instancia:           instanciaID,
		InstanciaUsuarios:   instanciaUsuarios,
		InstanciaCategorias: instanciaCategorias,
	}
}
