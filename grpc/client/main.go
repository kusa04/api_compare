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
	list, _ := client.ListUsers(ctx, &pb.ListUsersRequest{})
	for _, u := range list.Users {
		fmt.Printf("  ID:%d Name:%s Email:%s Age:%d\n", u.Id, u.Name, u.Email, u.Age)
	}

	// 作成
	fmt.Println("\n--- CreateUser(name=Taro, email=taro@example.com, age=30) ---")
	created, _ := client.CreateUser(ctx, &pb.CreateUserRequest{
		Name: "Taro", Email: "taro@example.com", Age: 30,
	})
	fmt.Printf("  作成: ID:%d Name:%s\n", created.Id, created.Name)

	// 取得
	fmt.Printf("\n--- GetUser(id=%d) ---\n", created.Id)
	got, _ := client.GetUser(ctx, &pb.GetUserRequest{Id: created.Id})
	fmt.Printf("  取得: ID:%d Name:%s Email:%s Age:%d\n", got.Id, got.Name, got.Email, got.Age)

	// 更新
	fmt.Printf("\n--- UpdateUser(id=%d, name=Taro Yamada, age=31) ---\n", created.Id)
	updated, _ := client.UpdateUser(ctx, &pb.UpdateUserRequest{
		Id: created.Id, Name: "Taro Yamada", Email: "taro@example.com", Age: 31,
	})
	fmt.Printf("  更新: ID:%d Name:%s Age:%d\n", updated.Id, updated.Name, updated.Age)

	// 検索
	fmt.Println("\n--- SearchUsers(name=User) ---")
	results, _ := client.SearchUsers(ctx, &pb.SearchUsersRequest{Name: "User"})
	fmt.Printf("  検索結果: %d件\n", len(results.Users))
	for _, u := range results.Users {
		fmt.Printf("  ID:%d Name:%s\n", u.Id, u.Name)
	}

	// 削除
	fmt.Printf("\n--- DeleteUser(id=%d) ---\n", created.Id)
	del, _ := client.DeleteUser(ctx, &pb.DeleteUserRequest{Id: created.Id})
	fmt.Printf("  削除: success=%v\n", del.Success)

	// 削除後の一覧
	fmt.Println("\n--- ListUsers() ---")
	list2, _ := client.ListUsers(ctx, &pb.ListUsersRequest{})
	fmt.Printf("  残り: %d件\n", len(list2.Users))
}
