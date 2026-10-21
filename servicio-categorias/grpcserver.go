package main

// grpcserver.go expone el servicio por gRPC, en paralelo a la API HTTP.
//
// El contrato esta en proto/categorias.proto y el codigo de este paquete pb
// se genera con protoc. Los metodos de aqui no tocan SQL: le piden los
// datos al repositorio, igual que los handlers HTTP.

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "servicio-categorias/pb/categoriaspb"
)

type servidorCategorias struct {
	pb.UnimplementedCategoriasServer
}

func (s *servidorCategorias) Listar(ctx context.Context, _ *pb.ListarRequest) (*pb.ListarResponse, error) {
	lista := store.GetAll()

	salida := make([]*pb.Categoria, 0, len(lista))
	for _, c := range lista {
		salida = append(salida, &pb.Categoria{
			Id:     int32(c.ID),
			Nombre: c.Nombre,
			Color:  c.Color,
		})
	}

	return &pb.ListarResponse{Categorias: salida, Instancia: instanciaID}, nil
}

func (s *servidorCategorias) Obtener(ctx context.Context, req *pb.ObtenerRequest) (*pb.ObtenerResponse, error) {
	c, ok := store.Get(int(req.GetId()))
	if !ok {
		return &pb.ObtenerResponse{Encontrado: false, Instancia: instanciaID}, nil
	}

	return &pb.ObtenerResponse{
		Categoria:  &pb.Categoria{Id: int32(c.ID), Nombre: c.Nombre, Color: c.Color},
		Encontrado: true,
		Instancia:  instanciaID,
	}, nil
}

func (s *servidorCategorias) Contar(ctx context.Context, _ *pb.ContarRequest) (*pb.ContarResponse, error) {
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
	pb.RegisterCategoriasServer(servidor, &servidorCategorias{})

	// La reflexion deja que grpcurl descubra los metodos sin tener
	// que pasarle el archivo .proto.
	reflection.Register(servidor)

	log.Printf("servidor grpc categorias [%s] escuchando en :%s", instanciaID, puerto)
	if err := servidor.Serve(escucha); err != nil {
		log.Fatalf("el servidor grpc fallo: %v", err)
	}
}
