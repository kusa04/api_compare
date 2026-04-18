package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	pb "api_compare/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RawResult は1ラウンド分の計測生データ。
type RawResult struct {
	Durations []time.Duration
	Elapsed   time.Duration
}

// Result はベンチマーク1テスト（全ラウンド集約後）の集計値。
//   - Elapsed: 全リクエスト完了までの wall clock 時間の合計（スループット指標）
//   - Average / P50 / P95 / P99 / Min / Max: 個別リクエストレイテンシ
type Result struct {
	Elapsed time.Duration
	Average time.Duration
	P50     time.Duration
	P95     time.Duration
	P99     time.Duration
	Min     time.Duration
	Max     time.Duration
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	// nearest-rank 法
	rank := int(p * float64(len(sorted)-1))
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func calcResult(durations []time.Duration, elapsed time.Duration) Result {
	n := len(durations)
	sum := time.Duration(0)
	for _, d := range durations {
		sum += d
	}
	sorted := make([]time.Duration, n)
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return Result{
		Elapsed: elapsed,
		Average: sum / time.Duration(n),
		P50:     percentile(sorted, 0.50),
		P95:     percentile(sorted, 0.95),
		P99:     percentile(sorted, 0.99),
		Min:     sorted[0],
		Max:     sorted[n-1],
	}
}

// --- HTTP クライアントのフェアな設定 ---
//
// Go `http.Transport` のデフォルト `MaxIdleConnsPerHost=2` は HTTP/1.1 の
// プロトコル制約ではなくライブラリ固有の値。そのままだと並行リクエスト時に
// 2 本の TCP 接続に詰め込まれ、gRPC (HTTP/2 多重化) との比較が
// 「プロトコル差」ではなく「クライアント設定差」になってしまう。
// 並行数以上に引き上げて HTTP/1.1 が本来張れる並列コネクションを出せる状態にする。
func newHTTPClient(concurrency int) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        concurrency * 2,
		MaxIdleConnsPerHost: concurrency,
		MaxConnsPerHost:     concurrency,
		IdleConnTimeout:     90 * time.Second,
	}
	return &http.Client{Transport: transport}
}

// drainAndClose は Keep-Alive コネクションを再利用可能にするため
// Body を完全に読み切ってから Close する。
func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

// --- シングルリクエスト (GetUser) ---

func benchGRPCSingle(n int) RawResult {
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
		_ = resp.Id
		durations[i] = time.Since(t)
	}
	return RawResult{Durations: durations, Elapsed: time.Since(start)}
}

func benchRESTSingle(n int, concurrency int) RawResult {
	httpClient := newHTTPClient(concurrency)
	// ウォームアップ
	resp, err := httpClient.Get("http://localhost:8080/users/1")
	if err != nil {
		panic(err)
	}
	drainAndClose(resp.Body)

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
		drainAndClose(resp.Body)
		durations[i] = time.Since(t)
	}
	return RawResult{Durations: durations, Elapsed: time.Since(start)}
}

// --- 大きいペイロード (ListUsers: 100件) ---

func benchGRPCList(n int) RawResult {
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
		_ = resp.Users
		durations[i] = time.Since(t)
	}
	return RawResult{Durations: durations, Elapsed: time.Since(start)}
}

func benchRESTList(n int, concurrency int) RawResult {
	httpClient := newHTTPClient(concurrency)
	// ウォームアップ
	resp, err := httpClient.Get("http://localhost:8080/users?limit=100")
	if err != nil {
		panic(err)
	}
	drainAndClose(resp.Body)

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
		resp, err := httpClient.Get("http://localhost:8080/users?limit=100")
		if err != nil {
			panic(err)
		}
		var users []User
		if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
			panic(err)
		}
		drainAndClose(resp.Body)
		durations[i] = time.Since(t)
	}
	return RawResult{Durations: durations, Elapsed: time.Since(start)}
}

// --- 同時並行リクエスト ---

func benchGRPCConcurrent(n int, concurrency int) RawResult {
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
			resp, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: int32(idx%100 + 1)})
			if err != nil {
				panic(err)
			}
			_ = resp.Id
			durations[idx] = time.Since(t)
		}(i)
	}
	wg.Wait()
	return RawResult{Durations: durations, Elapsed: time.Since(start)}
}

func benchRESTConcurrent(n int, concurrency int) RawResult {
	httpClient := newHTTPClient(concurrency)
	// ウォームアップ
	resp, err := httpClient.Get("http://localhost:8080/users/1")
	if err != nil {
		panic(err)
	}
	drainAndClose(resp.Body)

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
			resp, err := httpClient.Get(fmt.Sprintf("http://localhost:8080/users/%d", idx%100+1))
			if err != nil {
				panic(err)
			}
			var u User
			if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
				panic(err)
			}
			drainAndClose(resp.Body)
			durations[idx] = time.Since(t)
		}(i)
	}
	wg.Wait()
	return RawResult{Durations: durations, Elapsed: time.Since(start)}
}

// --- ラウンド管理 ---
//
// gRPC → REST の順序バイアスを緩和するため、rounds 回繰り返し、
// ラウンドごとに実行順を交互に切り替える。全ラウンドの durations を集約。
func runTest(rounds int, grpcFn, restFn func() RawResult) (Result, Result) {
	var grpcAll, restAll RawResult
	for r := 0; r < rounds; r++ {
		var g, rr RawResult
		if r%2 == 0 {
			g = grpcFn()
			rr = restFn()
		} else {
			rr = restFn()
			g = grpcFn()
		}
		grpcAll.Durations = append(grpcAll.Durations, g.Durations...)
		grpcAll.Elapsed += g.Elapsed
		restAll.Durations = append(restAll.Durations, rr.Durations...)
		restAll.Elapsed += rr.Elapsed
	}
	return calcResult(grpcAll.Durations, grpcAll.Elapsed), calcResult(restAll.Durations, restAll.Elapsed)
}

// --- 表示 ---

func printResult(label string, grpcR, restR Result) {
	fmt.Printf("\n### %s ###\n\n", label)
	fmt.Printf("%-18s %15s %15s\n", "", "gRPC", "REST")
	fmt.Printf("%-18s %15s %15s\n", "----------", "----------", "----------")
	fmt.Printf("%-18s %15s %15s\n", "経過時間(全体)", grpcR.Elapsed, restR.Elapsed)
	fmt.Printf("%-18s %15s %15s\n", "平均(1件あたり)", grpcR.Average, restR.Average)
	fmt.Printf("%-18s %15s %15s\n", "p50", grpcR.P50, restR.P50)
	fmt.Printf("%-18s %15s %15s\n", "p95", grpcR.P95, restR.P95)
	fmt.Printf("%-18s %15s %15s\n", "p99", grpcR.P99, restR.P99)
	fmt.Printf("%-18s %15s %15s\n", "最小", grpcR.Min, restR.Min)
	fmt.Printf("%-18s %15s %15s\n", "最大", grpcR.Max, restR.Max)

	// p95 で比較（平均は外れ値に引っ張られやすいため）
	if grpcR.P95 < restR.P95 {
		fmt.Printf("\n→ gRPC が p95 レイテンシで %.2f 倍高速\n", float64(restR.P95)/float64(grpcR.P95))
	} else {
		fmt.Printf("\n→ REST が p95 レイテンシで %.2f 倍高速\n", float64(grpcR.P95)/float64(restR.P95))
	}
}

func main() {
	n := flag.Int("n", 1000, "1ラウンドあたりのリクエスト回数")
	c := flag.Int("c", 50, "同時並行数")
	rounds := flag.Int("rounds", 3, "各テストのラウンド数（順序バイアス緩和のため gRPC/REST の実行順を毎ラウンド交互に切り替える）")
	flag.Parse()

	fmt.Printf("=== gRPC vs REST API ベンチマーク ===\n")
	fmt.Printf("リクエスト数: %d / 同時並行数: %d / ラウンド数: %d\n", *n, *c, *rounds)
	fmt.Printf("（REST 側 http.Transport: MaxIdleConnsPerHost=%d, MaxConnsPerHost=%d）\n", *c, *c)

	// テスト1: シングルリクエスト（小さいペイロード）
	fmt.Println("\n[1/3] シングルリクエスト (GetUser) ベンチマーク中...")
	g1, r1 := runTest(*rounds,
		func() RawResult { return benchGRPCSingle(*n) },
		func() RawResult { return benchRESTSingle(*n, *c) },
	)
	printResult("テスト1: シングルリクエスト (1ユーザー)", g1, r1)

	// テスト2: 大きいペイロード
	fmt.Println("\n[2/3] 大きいペイロード (ListUsers: 100件) ベンチマーク中...")
	g2, r2 := runTest(*rounds,
		func() RawResult { return benchGRPCList(*n) },
		func() RawResult { return benchRESTList(*n, *c) },
	)
	printResult("テスト2: 大きいペイロード (100ユーザー)", g2, r2)

	// テスト3: 同時並行リクエスト
	fmt.Printf("\n[3/3] 同時並行リクエスト (並行数: %d) ベンチマーク中...\n", *c)
	g3, r3 := runTest(*rounds,
		func() RawResult { return benchGRPCConcurrent(*n, *c) },
		func() RawResult { return benchRESTConcurrent(*n, *c) },
	)
	printResult(fmt.Sprintf("テスト3: 同時並行リクエスト (並行数: %d)", *c), g3, r3)
}
