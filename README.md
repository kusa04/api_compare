# gRPC vs REST API 比較

gRPC と REST API の**パフォーマンス**と**設計思想の違い**を比較するプロジェクトです。
両サーバーとも同じ CRUD 操作（ユーザーの取得・一覧・作成・更新・削除・検索）を提供し、プロトコルと設計アプローチの違いを実際のコードで確認できます。

## 前提知識: gRPC と REST API の違い

| | gRPC | REST API |
|---|---|---|
| プロトコル | HTTP/2 | HTTP/1.1 |
| データ形式 | Protocol Buffers（バイナリ） | JSON（テキスト） |
| 型定義 | `.proto` ファイルで厳密に定義 | 規約ベース（OpenAPI等は任意） |
| コード生成 | `protoc` で自動生成 | 手動で実装 |
| 設計思想 | **関数（動詞）** 中心 | **リソース（名詞）** 中心 |

### 設計思想の違い: 動詞 vs 名詞

同じ CRUD 操作でも、API の表現方法が根本的に異なります。

| 操作 | gRPC（関数名 = 動詞） | REST（URL = 名詞 + HTTPメソッド = 動詞） |
|---|---|---|
| 取得 | `GetUser(id=4)` | `GET /users/4` |
| 一覧 | `ListUsers()` | `GET /users` |
| 作成 | `CreateUser(name=Taro, ...)` | `POST /users` + body |
| 更新 | `UpdateUser(id=4, name=..., ...)` | `PUT /users/4` + body |
| 削除 | `DeleteUser(id=4)` | `DELETE /users/4` |
| 検索 | `SearchUsers(name=User)` | `GET /users?name=User` |

- **gRPC**: 操作ごとに別の関数を定義する。何をするかは**関数名**が表現する
- **REST**: URL はリソースの場所（名詞）を指し、何をするかは **HTTPメソッド**（GET/POST/PUT/DELETE）が表現する。同じ `/users/4` でもメソッドによって取得・更新・削除が切り替わる

## ディレクトリ構成

```
api_compare/
├── grpc/
│   ├── proto/
│   │   └── user.proto            # サービスとメッセージの型定義（CRUD全操作）
│   ├── pb/
│   │   ├── user.pb.go            # protoc が自動生成（メッセージ型）
│   │   └── user_grpc.pb.go       # protoc が自動生成（サーバー/クライアントのインターフェース）
│   ├── server/
│   │   └── main.go               # gRPC サーバー（port 50051）
│   └── client/
│       └── main.go               # gRPC クライアント（CRUD デモ）
├── rest/
│   ├── server/
│   │   └── main.go               # REST API サーバー（port 8080）
│   └── client/
│       └── main.go               # REST API クライアント（CRUD デモ）
├── benchmark/
│   └── main.go                   # パフォーマンスベンチマーク
├── go.mod
└── go.sum
```

## セットアップ

### 必要なもの

- Go 1.21 以上
- protoc（Protocol Buffers コンパイラ）
- protoc の Go プラグイン

```bash
# protoc の Go プラグインをインストール
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 依存パッケージの取得
go mod tidy
```

### .proto からGoコードを再生成する場合

```bash
protoc --go_out=. --go_opt=paths=import \
       --go-grpc_out=. --go-grpc_opt=paths=import \
       -I grpc/proto grpc/proto/user.proto
```

## 使い方

### CRUD デモ（設計思想の違いを確認）

ターミナルを3つ開いて、それぞれで以下を実行します。

```bash
# ターミナル1: gRPC サーバー起動
go run grpc/server/main.go

# ターミナル2: REST サーバー起動
go run rest/server/main.go

# ターミナル3: クライアント実行
go run grpc/client/main.go   # gRPC の CRUD デモ
go run rest/client/main.go   # REST の CRUD デモ
```

### ベンチマーク（パフォーマンス比較）

両サーバーが起動した状態で:

```bash
go run benchmark/main.go -n 1000 -c 50 -rounds 3
```

- `-n`: 1ラウンドあたりのリクエスト回数（デフォルト: 1000）
- `-c`: 同時並行数（デフォルト: 50）
- `-rounds`: 各テストのラウンド数（デフォルト: 3）。ラウンドごとに gRPC/REST の実行順を交互に切り替え、ウォームアップ状態の違いによる順序バイアスを緩和する。

3種類のテストを実行します:
1. **シングルリクエスト** — 単純な1件取得の速度比較
2. **大きいペイロード** — 100件のユーザーリストの速度比較
3. **同時並行リクエスト** — 100件のユーザーに対する並行取得で HTTP/2 多重化の効果を測定

計測指標は **平均・p50・p95・p99・最小・最大**。テイルレイテンシを可視化するため、優劣判定は平均ではなく p95 で行っています。

---

## スクリプト解説

### 1. Proto定義 (`grpc/proto/user.proto`)

gRPC ではまず `.proto` ファイルでAPIの「契約」を定義します。

```protobuf
service UserService {
  rpc GetUser(GetUserRequest) returns (UserResponse);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc CreateUser(CreateUserRequest) returns (UserResponse);
  rpc UpdateUser(UpdateUserRequest) returns (UserResponse);
  rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
  rpc SearchUsers(SearchUsersRequest) returns (ListUsersResponse);
}
```

- **service**: 「どんなAPIがあるか」を定義。各操作が独立した RPC メソッドとして宣言される
- **message**: リクエスト・レスポンスの型を定義。Go の構造体のようなもの
- **`= 1`, `= 2`**: フィールド番号。JSON のキー名に相当するが、バイナリ通信では名前ではなくこの番号でデータを識別する

この `.proto` から `protoc` コマンドで Go のコードが自動生成されます（`pb/` ディレクトリ内）。
自動生成されたコードには、サーバー側が実装すべきインターフェースやクライアントの呼び出し関数が含まれます。

### 2. gRPC サーバー (`grpc/server/main.go`)

```go
type userServer struct {
    pb.UnimplementedUserServiceServer  // ← 自動生成されたインターフェースを埋め込み
    mu     sync.Mutex
    users  map[int32]*pb.UserResponse  // ← インメモリDB
}

// 各操作 = 独立した関数
func (s *userServer) GetUser(ctx, req)    → IDで1件取得
func (s *userServer) CreateUser(ctx, req) → 新規作成してIDを採番
func (s *userServer) UpdateUser(ctx, req) → IDを指定して更新
func (s *userServer) DeleteUser(ctx, req) → IDを指定して削除
func (s *userServer) SearchUsers(ctx, req) → 名前で部分一致検索
```

**ポイント:**
- `pb.UnimplementedUserServiceServer` を埋め込むことで、proto で定義したサービスを実装する構造体になる
- 操作ごとに別のメソッドをオーバーライドする。**1つの関数 = 1つの操作**
- `grpc.NewServer()` で gRPC サーバーを作り、`:50051` ポートでリッスン
- リクエスト・レスポンスは Protocol Buffers 形式（バイナリ）で自動的にシリアライズ/デシリアライズされる

### 3. REST サーバー (`rest/server/main.go`)

```go
// URL（名詞）でルーティングし、HTTPメソッド（動詞）で処理を分岐
func usersHandler(w, r) {   // /users
    switch r.Method {
    case "GET":    → 一覧取得 or 検索（?name=xxx）
    case "POST":   → 新規作成（bodyのJSONから）
    }
}

func userHandler(w, r) {    // /users/{id}
    switch r.Method {
    case "GET":    → 1件取得
    case "PUT":    → 更新
    case "DELETE": → 削除
    }
}
```

**ポイント:**
- Go 標準ライブラリの `net/http` だけで実装（外部ライブラリ不要）
- **1つのURL + HTTPメソッドの組み合わせ = 1つの操作**。同じ `/users/{id}` でもメソッドで動作が変わる
- レスポンスは `json.NewEncoder` で構造体 → JSON テキストに変換して返す
- gRPC と異なり、型定義ファイルからの自動生成はなく、構造体を手動で定義している

### 4. gRPC クライアント (`grpc/client/main.go`)

```go
conn, _ := grpc.NewClient("localhost:50051", ...)
client := pb.NewUserServiceClient(conn)

client.ListUsers(ctx, &pb.ListUsersRequest{})                        // 一覧
client.CreateUser(ctx, &pb.CreateUserRequest{Name: "Taro", ...})     // 作成
client.GetUser(ctx, &pb.GetUserRequest{Id: 4})                       // 取得
client.UpdateUser(ctx, &pb.UpdateUserRequest{Id: 4, Name: "...", ...}) // 更新
client.SearchUsers(ctx, &pb.SearchUsersRequest{Name: "User"})        // 検索
client.DeleteUser(ctx, &pb.DeleteUserRequest{Id: 4})                 // 削除
```

**ポイント:**
- `pb.NewUserServiceClient` は proto から自動生成されたクライアント。型安全にメソッドを呼べる
- まるでローカル関数を呼ぶように RPC を呼び出せる。URLを意識する必要がない

### 5. REST クライアント (`rest/client/main.go`)

```go
http.Get("http://localhost:8080/users")                              // GET    /users       → 一覧
http.Post("http://localhost:8080/users", body)                       // POST   /users       → 作成
http.Get("http://localhost:8080/users/4")                            // GET    /users/4     → 取得
http.NewRequest("PUT", "http://localhost:8080/users/4", body)        // PUT    /users/4     → 更新
http.Get("http://localhost:8080/users?name=User")                    // GET    /users?name= → 検索
http.NewRequest("DELETE", "http://localhost:8080/users/4", nil)      // DELETE /users/4     → 削除
```

**ポイント:**
- URLの構築・JSONのパースを手動で行う必要がある
- 同じ `/users/4` に対して GET/PUT/DELETE とメソッドを変えることで操作を切り替える

### 6. ベンチマーク (`benchmark/main.go`)

```go
for i := 0; i < n; i++ {
    t := time.Now()
    client.GetUser(...)           // ← 1回のリクエスト
    durations[i] = time.Since(t)  // ← かかった時間を記録
}
```

**ポイント:**
- 計測前に1回「ウォームアップ」リクエストを送る（初回は接続確立のコストが含まれるため除外）
- 各リクエストの所要時間を個別に記録し、**経過時間(全体)・平均・p50・p95・p99・最小・最大** を算出
  - **経過時間(全体)**: 全リクエストが完了するまでの wall clock 時間（スループット指標）
  - **平均・p50/p95/p99**: 個別リクエスト所要時間の分布（外れ値に引っ張られにくい p95 を優劣判定に使用）
- REST 側はレスポンスの JSON デコードまでを、gRPC 側は自動生成コードによる Protobuf デコード完了までを計測範囲に含め、条件を揃えている
- 3種類のテスト: シングル、大きいペイロード（100件）、同時並行（goroutine + セマフォ制御で100件のユーザーに分散リクエスト）

#### 計測の公平性のための工夫

本ベンチマークでは「プロトコル差」を測りたいため、以下の工夫を入れています。

| 観点 | 対策 |
|---|---|
| **`http.Transport` のデフォルト `MaxIdleConnsPerHost=2`** は Go 固有の制約で HTTP/1.1 の仕様ではない。そのままだと REST 側が 2 本の TCP 接続に詰め込まれ、gRPC (HTTP/2 多重化) と比べて構造的に不利になる。 | `MaxIdleConnsPerHost` / `MaxConnsPerHost` を並行数に合わせて明示設定し、HTTP/1.1 本来の並列接続を張れる状態にする。 |
| **gRPC→REST の直列実行** だとウォームアップ状態に順序バイアスが入る。 | `-rounds N` で複数ラウンド回し、ラウンドごとに実行順を交互に切り替えて durations を集約する。 |
| **サーバ側が読み取りでも排他ロック** だと並行リクエストがサーバの mutex で直列化され、プロトコル差ではなくサーバ競合を測ってしまう。 | `grpc/server`, `rest/server` とも `sync.RWMutex` を使い、読み取り系 (`GetUser`, `ListUsers`, `SearchUsers`, `GET /users`, `GET /users/{id}`) は `RLock`、書き込み系だけ `Lock` にする。 |
| **`ListUsers` の `Count` が無視されていた** ため「100件返っているのは偶然」の状態だった。 | gRPC サーバは `req.Count` を尊重（0 なら全件、正なら ID 昇順で先頭 N 件）。REST も `GET /users?limit=N` をサポートし、ベンチマーク側で `?limit=100` を使用。 |
| **平均・最小・最大のみだと外れ値に敏感**。 | p50 / p95 / p99 を出力し、優劣判定は p95 で行う。 |
| **Keep-Alive の再利用が不完全** だと計測にブレが出る。 | REST 側は `io.Copy(io.Discard, resp.Body)` で Body をドレインしてから Close する。 |

## ベンチマーク結果（実測）

`go run benchmark/main.go -n 1000 -c 50 -rounds 3` の実行結果の一例:

（REST 側 http.Transport: MaxIdleConnsPerHost=50, MaxConnsPerHost=50）

### テスト1: シングルリクエスト (1ユーザー取得)

|  | gRPC | REST |
|---|---|---|
| 経過時間(全体) | 131.26ms | 96.28ms |
| 平均(1件あたり) | 43.7µs | 32.1µs |
| p50 | 38.5µs | 31.2µs |
| p95 | 78.5µs | 38.5µs |
| p99 | 107.2µs | 52.9µs |
| 最小 | 29.1µs | 23.5µs |
| 最大 | 381.7µs | 271.6µs |

**→ REST が p95 レイテンシで 2.04 倍高速**（ペイロードが極めて小さい場合は、HTTP/1.1 + keep-alive でもオーバーヘッドが小さく、gRPC の HTTP/2 フレーミングコストが相対的に目立つ）

### テスト2: 大きいペイロード (100ユーザーリスト)

|  | gRPC | REST |
|---|---|---|
| 経過時間(全体) | 177.94ms | 354.21ms |
| 平均(1件あたり) | 59.3µs | 118.0µs |
| p50 | 56.8µs | 115.4µs |
| p95 | 71.0µs | 126.0µs |
| p99 | 125.3µs | 202.2µs |
| 最小 | 45.1µs | 103.3µs |
| 最大 | 206.2µs | 435.6µs |

**→ gRPC が p95 レイテンシで 1.77 倍高速**（ペイロードが大きくなると Protobuf のバイナリ効率が JSON デコードコストを上回る）

### テスト3: 同時並行リクエスト (並行数: 50、100ユーザーに分散)

|  | gRPC | REST |
|---|---|---|
| 経過時間(全体) | 22.97ms | 31.14ms |
| 平均(1件あたり) | 357.6µs | 503.3µs |
| p50 | 330.2µs | 419.3µs |
| p95 | 623.4µs | 1.26ms |
| p99 | 721.7µs | 1.89ms |
| 最小 | 69.3µs | 35.9µs |
| 最大 | 897.0µs | 2.80ms |

**→ gRPC が p95 レイテンシで 2.02 倍高速**（HTTP/2 の多重化により、1 接続上で 50 並行リクエストが効率的にさばかれる。100件のユーザーに対してID 1〜100を分散リクエスト）

> 注: 実測値は実行環境・CPU 負荷・サーバとクライアントが同一プロセス内か別マシンか等で大きく変動します。必ず各自の環境で再実行し、**傾向**として解釈してください。

## なぜ gRPC が速くなりやすいのか（そして常に速いとは限らない理由）

**gRPC が有利になる要因:**
1. **バイナリ vs テキスト**: Protocol Buffers はバイナリ形式なので、JSON よりシリアライズ/デシリアライズが高速かつデータサイズが小さい（**ペイロードが大きいほど効果大**）
2. **HTTP/2 の多重化**: gRPC は HTTP/2 を使い、1つの接続上で複数リクエストを並行して多重化できる。**並行リクエストでの差が大きい**のはこれが理由
3. **コード生成**: 型やメソッドが事前にコンパイルされているため、実行時のリフレクションが不要

**REST（HTTP/1.1 + JSON）が健闘するケース:**
- ペイロードが極めて小さい単発リクエストでは、gRPC の HTTP/2 フレーミングや Protobuf のオーバーヘッドが相対的に目立ち、差が縮むか逆転することがある
- Go の `net/http` は keep-alive が有効なので、同一ホストへの連続リクエストでは接続確立コストが償却される

→ **API 選定は「何を最適化したいか（単発レイテンシ / スループット / 可読性 / ツーリング）」で判断するべきで、gRPC が常に速いわけではない**点に注意。
