package main

import (
	"context"
	"fmt"
	"log"

	pb "api_compare/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	ctx := context.Background()

	fmt.Println("=== gRPC クライアント: 関数（動詞）でAPIを呼び出す ===")
	fmt.Println()

	// 一覧取得
	fmt.Println("--- ListUsers() ---")
	list, err := client.ListUsers(ctx, &pb.ListUsersRequest{})
	if err != nil {
		log.Fatalf("ListUsers failed: %v", err)
	}
	for _, u := range list.Users {
		fmt.Printf("  ID:%d Name:%s Email:%s Age:%d\n", u.Id, u.Name, u.Email, u.Age)
	}

	// 作成
	fmt.Println("\n--- CreateUser(name=Taro, email=taro@example.com, age=30) ---")
	created, err := client.CreateUser(ctx, &pb.CreateUserRequest{
		Name: "Taro", Email: "taro@example.com", Age: 30,
	})
	if err != nil {
		log.Fatalf("CreateUser failed: %v", err)
	}
	fmt.Printf("  作成: ID:%d Name:%s\n", created.Id, created.Name)

	// 取得
	fmt.Printf("\n--- GetUser(id=%d) ---\n", created.Id)
	got, err := client.GetUser(ctx, &pb.GetUserRequest{Id: created.Id})
	if err != nil {
		log.Fatalf("GetUser failed: %v", err)
	}
	fmt.Printf("  取得: ID:%d Name:%s Email:%s Age:%d\n", got.Id, got.Name, got.Email, got.Age)

	// 更新
	fmt.Printf("\n--- UpdateUser(id=%d, name=Taro Yamada, age=31) ---\n", created.Id)
	updated, err := client.UpdateUser(ctx, &pb.UpdateUserRequest{
		Id: created.Id, Name: "Taro Yamada", Email: "taro@example.com", Age: 31,
	})
	if err != nil {
		log.Fatalf("UpdateUser failed: %v", err)
	}
	fmt.Printf("  更新: ID:%d Name:%s Age:%d\n", updated.Id, updated.Name, updated.Age)

	// 検索
	fmt.Println("\n--- SearchUsers(name=User) ---")
	results, err := client.SearchUsers(ctx, &pb.SearchUsersRequest{Name: "User"})
	if err != nil {
		log.Fatalf("SearchUsers failed: %v", err)
	}
	fmt.Printf("  検索結果: %d件\n", len(results.Users))
	for _, u := range results.Users {
		fmt.Printf("  ID:%d Name:%s\n", u.Id, u.Name)
	}

	// 削除
	fmt.Printf("\n--- DeleteUser(id=%d) ---\n", created.Id)
	del, err := client.DeleteUser(ctx, &pb.DeleteUserRequest{Id: created.Id})
	if err != nil {
		log.Fatalf("DeleteUser failed: %v", err)
	}
	fmt.Printf("  削除: success=%v\n", del.Success)

	// 削除後の一覧
	fmt.Println("\n--- ListUsers() ---")
	list2, err := client.ListUsers(ctx, &pb.ListUsersRequest{})
	if err != nil {
		log.Fatalf("ListUsers failed: %v", err)
	}
	fmt.Printf("  残り: %d件\n", len(list2.Users))
}
