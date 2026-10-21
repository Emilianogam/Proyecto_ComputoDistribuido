package main

// grpcserver.go expone el servicio por gRPC, en paralelo a la API HTTP.
//
// Ademas del CRUD, implementa Resumen, que llama por gRPC a usuarios y a
// categorias pasando por el load balancer. Esa es la logica de
// comunicacion entre servicios.

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "servicio-tareas/pb/tareaspb"
)

type servidorTareas struct {
	pb.UnimplementedTareasServer
}

func (s *servidorTareas) Listar(ctx context.Context, _ *pb.ListarRequest) (*pb.ListarResponse, error) {
	lista := store.GetAll()

	salida := make([]*pb.Tarea, 0, len(lista))
	for _, t := range lista {
		salida = append(salida, &pb.Tarea{
			Id:         int32(t.ID),
			Titulo:     t.Titulo,
			Completada: t.Completada,
		})
	}

	return &pb.ListarResponse{Tareas: salida, Instancia: instanciaID}, nil
}

func (s *servidorTareas) Obtener(ctx context.Context, req *pb.ObtenerRequest) (*pb.ObtenerResponse, error) {
	t, ok := store.Get(int(req.GetId()))
	if !ok {
		return &pb.ObtenerResponse{Encontrado: false, Instancia: instanciaID}, nil
	}

	return &pb.ObtenerResponse{
		Tarea:      &pb.Tarea{Id: int32(t.ID), Titulo: t.Titulo, Completada: t.Completada},
		Encontrado: true,
		Instancia:  instanciaID,
	}, nil
}

func (s *servidorTareas) Contar(ctx context.Context, _ *pb.ContarRequest) (*pb.ContarResponse, error) {
	return &pb.ContarResponse{
		Total:     int32(len(store.GetAll())),
		Instancia: instanciaID,
	}, nil
}

// Resumen es la comunicacion entre servicios: para contestar, este servicio
// llama por gRPC a usuarios y a categorias a traves del load balancer.
func (s *servidorTareas) Resumen(ctx context.Context, _ *pb.ResumenRequest) (*pb.ResumenResponse, error) {
	d := armarResumen(ctx)

	return &pb.ResumenResponse{
		Usuarios:            d.Usuarios,
		Tareas:              d.Tareas,
		Categorias:          d.Categorias,
		Instancia:           d.Instancia,
		InstanciaUsuarios:   d.InstanciaUsuarios,
		InstanciaCategorias: d.InstanciaCategorias,
	}, nil
}

// arrancarGRPC levanta el servidor gRPC en su propio puerto.
func arrancarGRPC(puerto string) {
	escucha, err := net.Listen("tcp", ":"+puerto)
	if err != nil {
		log.Fatalf("no se pudo abrir el puerto grpc %s: %v", puerto, err)
	}

	servidor := grpc.NewServer()
	pb.RegisterTareasServer(servidor, &servidorTareas{})

	// La reflexion deja que grpcurl descubra los metodos sin tener
	// que pasarle el archivo .proto.
	reflection.Register(servidor)

	log.Printf("servidor grpc tareas [%s] escuchando en :%s", instanciaID, puerto)
	if err := servidor.Serve(escucha); err != nil {
		log.Fatalf("el servidor grpc fallo: %v", err)
	}
}
