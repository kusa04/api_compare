package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	pb "api_compare/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Result struct {
	Total   time.Duration
	Average time.Duration
	Min     time.Duration
	Max     time.Duration
	P50     time.Duration
	P99     time.Duration
}

func calcResult(durations []time.Duration, total time.Duration) Result {
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	n := len(durations)
	return Result{
		Total:   total,
		Average: total / time.Duration(n),
		Min:     durations[0],
		Max:     durations[n-1],
		P50:     durations[n*50/100],
		P99:     durations[n*99/100],
	}
}

// --- シングルリクエスト (GetUser) ---

func benchGRPCSingle(n int) Result {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1})

	durations := make([]time.Duration, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.Now()
		_, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1})
		durations[i] = time.Since(t)
		if err != nil {
			panic(err)
		}
	}
	return calcResult(durations, time.Since(start))
}

func benchRESTSingle(n int) Result {
	httpClient := &http.Client{}
	resp, _ := httpClient.Get("http://localhost:8080/user?id=1")
	if resp != nil {
		resp.Body.Close()
	}

	durations := make([]time.Duration, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.Now()
		resp, err := httpClient.Get("http://localhost:8080/user?id=1")
		durations[i] = time.Since(t)
		if err != nil {
			panic(err)
		}
		resp.Body.Close()
	}
	return calcResult(durations, time.Since(start))
}

// --- 大きいペイロード (ListUsers: 100件) ---

func benchGRPCList(n int) Result {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	client.ListUsers(context.Background(), &pb.ListUsersRequest{Count: 100})

	durations := make([]time.Duration, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.Now()
		_, err := client.ListUsers(context.Background(), &pb.ListUsersRequest{Count: 100})
		durations[i] = time.Since(t)
		if err != nil {
			panic(err)
		}
	}
	return calcResult(durations, time.Since(start))
}

func benchRESTList(n int) Result {
	httpClient := &http.Client{}
	resp, _ := httpClient.Get("http://localhost:8080/users")
	if resp != nil {
		resp.Body.Close()
	}

	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	durations := make([]time.Duration, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.Now()
		resp, err := httpClient.Get("http://localhost:8080/users")
		durations[i] = time.Since(t)
		if err != nil {
			panic(err)
		}
		var users []User
		json.NewDecoder(resp.Body).Decode(&users)
		resp.Body.Close()
	}
	return calcResult(durations, time.Since(start))
}

// --- 同時並行リクエスト ---

func benchGRPCConcurrent(n int, concurrency int) Result {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1})

	durations := make([]time.Duration, n)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			t := time.Now()
			_, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1})
			d := time.Since(t)
			if err != nil {
				panic(err)
			}
			mu.Lock()
			durations[idx] = d
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return calcResult(durations, time.Since(start))
}

func benchRESTConcurrent(n int, concurrency int) Result {
	httpClient := &http.Client{}
	resp, _ := httpClient.Get("http://localhost:8080/user?id=1")
	if resp != nil {
		resp.Body.Close()
	}

	durations := make([]time.Duration, n)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			t := time.Now()
			resp, err := httpClient.Get("http://localhost:8080/user?id=1")
			d := time.Since(t)
			if err != nil {
				panic(err)
			}
			resp.Body.Close()
			mu.Lock()
			durations[idx] = d
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return calcResult(durations, time.Since(start))
}

// --- 表示 ---

func printResult(label string, grpcR, restR Result) {
	fmt.Printf("\n### %s ###\n\n", label)
	fmt.Printf("%-12s %15s %15s\n", "", "gRPC", "REST")
	fmt.Printf("%-12s %15s %15s\n", "----------", "----------", "----------")
	fmt.Printf("%-12s %15s %15s\n", "合計", grpcR.Total, restR.Total)
	fmt.Printf("%-12s %15s %15s\n", "平均", grpcR.Average, restR.Average)
	fmt.Printf("%-12s %15s %15s\n", "最小", grpcR.Min, restR.Min)
	fmt.Printf("%-12s %15s %15s\n", "最大", grpcR.Max, restR.Max)
	fmt.Printf("%-12s %15s %15s\n", "P50", grpcR.P50, restR.P50)
	fmt.Printf("%-12s %15s %15s\n", "P99", grpcR.P99, restR.P99)

	if grpcR.Average < restR.Average {
		fmt.Printf("\n→ gRPC が %.2f 倍高速\n", float64(restR.Average)/float64(grpcR.Average))
	} else {
		fmt.Printf("\n→ REST が %.2f 倍高速\n", float64(grpcR.Average)/float64(restR.Average))
	}
}

func main() {
	n := flag.Int("n", 1000, "リクエスト回数")
	c := flag.Int("c", 50, "同時並行数")
	flag.Parse()

	fmt.Printf("=== gRPC vs REST API ベンチマーク ===\n")
	fmt.Printf("リクエスト数: %d / 同時並行数: %d\n", *n, *c)

	// テスト1: シングルリクエスト（小さいペイロード）
	fmt.Println("\n[1/3] シングルリクエスト (GetUser) ベンチマーク中...")
	g1 := benchGRPCSingle(*n)
	r1 := benchRESTSingle(*n)
	printResult("テスト1: シングルリクエスト (1ユーザー)", g1, r1)

	// テスト2: 大きいペイロード
	fmt.Println("\n[2/3] 大きいペイロード (ListUsers: 100件) ベンチマーク中...")
	g2 := benchGRPCList(*n)
	r2 := benchRESTList(*n)
	printResult("テスト2: 大きいペイロード (100ユーザー)", g2, r2)

	// テスト3: 同時並行リクエスト
	fmt.Printf("\n[3/3] 同時並行リクエスト (並行数: %d) ベンチマーク中...\n", *c)
	g3 := benchGRPCConcurrent(*n, *c)
	r3 := benchRESTConcurrent(*n, *c)
	printResult(fmt.Sprintf("テスト3: 同時並行リクエスト (並行数: %d)", *c), g3, r3)
}
