package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	pb "api_compare/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Result はベンチマーク1セットの集計値を表す。
//   - Elapsed: 全リクエスト完了までの wall clock 時間（スループット指標）
//   - Average: 個別リクエスト所要時間の平均（レイテンシ指標）
type Result struct {
	Elapsed time.Duration
	Average time.Duration
	Min     time.Duration
	Max     time.Duration
}

func calcResult(durations []time.Duration, elapsed time.Duration) Result {
	n := len(durations)
	sum := time.Duration(0)
	min, max := durations[0], durations[0]
	for _, d := range durations {
		sum += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	return Result{
		Elapsed: elapsed,
		Average: sum / time.Duration(n),
		Min:     min,
		Max:     max,
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
	// ウォームアップ
	if _, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1}); err != nil {
		panic(err)
	}

	durations := make([]time.Duration, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.Now()
		resp, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1})
		if err != nil {
			panic(err)
		}
		_ = resp.Id // 戻り値を参照（REST 側と対称性を保つ）
		durations[i] = time.Since(t)
	}
	return calcResult(durations, time.Since(start))
}

func benchRESTSingle(n int) Result {
	httpClient := &http.Client{}
	// ウォームアップ
	resp, err := httpClient.Get("http://localhost:8080/users/1")
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

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
		resp, err := httpClient.Get("http://localhost:8080/users/1")
		if err != nil {
			panic(err)
		}
		var u User
		if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
			panic(err)
		}
		resp.Body.Close()
		durations[i] = time.Since(t)
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
	// ウォームアップ
	if _, err := client.ListUsers(context.Background(), &pb.ListUsersRequest{Count: 100}); err != nil {
		panic(err)
	}

	durations := make([]time.Duration, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		t := time.Now()
		resp, err := client.ListUsers(context.Background(), &pb.ListUsersRequest{Count: 100})
		if err != nil {
			panic(err)
		}
		_ = resp.Users // 戻り値を参照（REST 側のデコードと対称性を保つ）
		durations[i] = time.Since(t)
	}
	return calcResult(durations, time.Since(start))
}

func benchRESTList(n int) Result {
	httpClient := &http.Client{}
	// ウォームアップ
	resp, err := httpClient.Get("http://localhost:8080/users")
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

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
		if err != nil {
			panic(err)
		}
		var users []User
		if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
			panic(err)
		}
		resp.Body.Close()
		durations[i] = time.Since(t)
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
	// ウォームアップ
	if _, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1}); err != nil {
		panic(err)
	}

	durations := make([]time.Duration, n)
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
			resp, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: 1})
			if err != nil {
				panic(err)
			}
			_ = resp.Id
			durations[idx] = time.Since(t)
		}(i)
	}
	wg.Wait()
	return calcResult(durations, time.Since(start))
}

func benchRESTConcurrent(n int, concurrency int) Result {
	httpClient := &http.Client{}
	// ウォームアップ
	resp, err := httpClient.Get("http://localhost:8080/users/1")
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	durations := make([]time.Duration, n)
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
			resp, err := httpClient.Get("http://localhost:8080/users/1")
			if err != nil {
				panic(err)
			}
			var u User
			if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
				panic(err)
			}
			resp.Body.Close()
			durations[idx] = time.Since(t)
		}(i)
	}
	wg.Wait()
	return calcResult(durations, time.Since(start))
}

// --- 表示 ---

func printResult(label string, grpcR, restR Result) {
	fmt.Printf("\n### %s ###\n\n", label)
	fmt.Printf("%-16s %15s %15s\n", "", "gRPC", "REST")
	fmt.Printf("%-16s %15s %15s\n", "----------", "----------", "----------")
	fmt.Printf("%-16s %15s %15s\n", "経過時間(全体)", grpcR.Elapsed, restR.Elapsed)
	fmt.Printf("%-16s %15s %15s\n", "平均(1件あたり)", grpcR.Average, restR.Average)
	fmt.Printf("%-16s %15s %15s\n", "最小", grpcR.Min, restR.Min)
	fmt.Printf("%-16s %15s %15s\n", "最大", grpcR.Max, restR.Max)

	if grpcR.Average < restR.Average {
		fmt.Printf("\n→ gRPC が平均レイテンシで %.2f 倍高速\n", float64(restR.Average)/float64(grpcR.Average))
	} else {
		fmt.Printf("\n→ REST が平均レイテンシで %.2f 倍高速\n", float64(grpcR.Average)/float64(restR.Average))
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
