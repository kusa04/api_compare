package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	pb "api_compare/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userServer struct {
	pb.UnimplementedUserServiceServer
	mu     sync.Mutex
	users  map[int32]*pb.UserResponse
	nextID int32
}

func newUserServer() *userServer {
	s := &userServer{
		users:  make(map[int32]*pb.UserResponse),
		nextID: 1,
	}
	// 初期データ（ベンチマーク用に100件）
	for i := 0; i < 100; i++ {
		s.users[s.nextID] = &pb.UserResponse{
			Id:    s.nextID,
			Name:  fmt.Sprintf("User %d", s.nextID),
			Email: fmt.Sprintf("user%d@example.com", s.nextID),
			Age:   int32(20 + i*5),
		}
		s.nextID++
	}
	return s
}

func (s *userServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[req.Id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "user %d not found", req.Id)
	}
	return user, nil
}

func (s *userServer) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	users := make([]*pb.UserResponse, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return &pb.ListUsersResponse{Users: users}, nil
}

func (s *userServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.UserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user := &pb.UserResponse{
		Id:    s.nextID,
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	}
	s.users[s.nextID] = user
	s.nextID++
	return user, nil
}

func (s *userServer) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[req.Id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "user %d not found", req.Id)
	}
	user.Name = req.Name
	user.Email = req.Email
	user.Age = req.Age
	return user, nil
}

func (s *userServer) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[req.Id]; !ok {
		return nil, status.Errorf(codes.NotFound, "user %d not found", req.Id)
	}
	delete(s.users, req.Id)
	return &pb.DeleteUserResponse{Success: true}, nil
}

func (s *userServer) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.ListUsersResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var results []*pb.UserResponse
	for _, u := range s.users {
		if strings.Contains(strings.ToLower(u.Name), strings.ToLower(req.Name)) {
			results = append(results, u)
		}
	}
	return &pb.ListUsersResponse{Users: results}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, newUserServer())

	fmt.Println("gRPC server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
