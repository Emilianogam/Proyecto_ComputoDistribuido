package main

// grpcserver.go expone el servicio por gRPC, en paralelo a la API HTTP.
//
// El contrato esta en proto/usuarios.proto y el codigo de este paquete pb
// se genera con protoc. Los metodos de aqui no tocan SQL: le piden los
// datos al repositorio, igual que los handlers HTTP.

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "servicio-usuarios/pb/usuariospb"
)

type servidorUsuarios struct {
	pb.UnimplementedUsuariosServer
}

func (s *servidorUsuarios) Listar(ctx context.Context, _ *pb.ListarRequest) (*pb.ListarResponse, error) {
	lista := store.GetAll()

	salida := make([]*pb.Usuario, 0, len(lista))
	for _, u := range lista {
		salida = append(salida, &pb.Usuario{
			Id:     int32(u.ID),
			Nombre: u.Nombre,
			Email:  u.Email,
		})
	}

	return &pb.ListarResponse{Usuarios: salida, Instancia: instanciaID}, nil
}

func (s *servidorUsuarios) Obtener(ctx context.Context, req *pb.ObtenerRequest) (*pb.ObtenerResponse, error) {
	u, ok := store.Get(int(req.GetId()))
	if !ok {
		return &pb.ObtenerResponse{Encontrado: false, Instancia: instanciaID}, nil
	}

	return &pb.ObtenerResponse{
		Usuario:    &pb.Usuario{Id: int32(u.ID), Nombre: u.Nombre, Email: u.Email},
		Encontrado: true,
		Instancia:  instanciaID,
	}, nil
}

func (s *servidorUsuarios) Contar(ctx context.Context, _ *pb.ContarRequest) (*pb.ContarResponse, error) {
	return &pb.ContarResponse{
		Total:     int32(len(store.GetAll())),
		Instancia: instanciaID,
	}, nil
}

// arrancarGRPC levanta el servidor gRPC en su propio puerto.
func arrancarGRPC(puerto string) {
	escucha, err := net.Listen("tcp", ":"+puerto)
	if err != nil {
		log.Fatalf("no se pudo abrir el puerto grpc %s: %v", puerto, err)
	}

	servidor := grpc.NewServer()
	pb.RegisterUsuariosServer(servidor, &servidorUsuarios{})

	// La reflexion deja que grpcurl descubra los metodos sin tener
	// que pasarle el archivo .proto.
	reflection.Register(servidor)

	log.Printf("servidor grpc usuarios [%s] escuchando en :%s", instanciaID, puerto)
	if err := servidor.Serve(escucha); err != nil {
		log.Fatalf("el servidor grpc fallo: %v", err)
	}
}
