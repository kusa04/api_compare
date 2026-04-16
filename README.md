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
go run benchmark/main.go -n 1000 -c 50
```

- `-n`: リクエスト回数（デフォルト: 1000）
- `-c`: 同時並行数（デフォルト: 50）

3種類のテストを実行します:
1. **シングルリクエスト** — 単純な1件取得の速度比較
2. **大きいペイロード** — 100件のユーザーリストの速度比較
3. **同時並行リクエスト** — HTTP/2 多重化の効果を測定

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
- 各リクエストの所要時間を個別に記録し、合計・平均・最小・最大・P50・P99 を算出
- **P50**: 全リクエストの50%がこの時間以内に完了（中央値）
- **P99**: 全リクエストの99%がこの時間以内に完了（最悪ケースに近い値）
- 3種類のテスト: シングル、大きいペイロード（100件）、同時並行（goroutine + セマフォ制御）

## ベンチマーク結果（実測）

`go run benchmark/main.go -n 1000 -c 50` の実行結果:

### テスト1: シングルリクエスト (1ユーザー取得)

|  | gRPC | REST |
|---|---|---|
| 合計 | 45.6ms | 205.3ms |
| 平均 | 45.6µs | 205.3µs |
| 最小 | 29.5µs | 166.1µs |
| 最大 | 297.9µs | 409.8µs |
| P50 | 42.2µs | 197.4µs |
| P99 | 86.9µs | 281.6µs |

**→ gRPC が 4.50 倍高速**

### テスト2: 大きいペイロード (100ユーザーリスト)

|  | gRPC | REST |
|---|---|---|
| 合計 | 59.2ms | 113.4ms |
| 平均 | 59.2µs | 113.4µs |
| 最小 | 45.7µs | 35.3µs |
| 最大 | 282.7µs | 283.1µs |
| P50 | 56.3µs | 43.1µs |
| P99 | 155.3µs | 73.4µs |

**→ gRPC が 1.91 倍高速**（合計では gRPC が速いが、P50/P99 では REST が健闘）

### テスト3: 同時並行リクエスト (並行数: 50)

|  | gRPC | REST |
|---|---|---|
| 合計 | 7.0ms | 30.7ms |
| 平均 | 7.0µs | 30.7µs |
| 最小 | 90.3µs | 226.4µs |
| 最大 | 965.7µs | 3.1ms |
| P50 | 302.3µs | 1.4ms |
| P99 | 775.4µs | 2.5ms |

**→ gRPC が 4.40 倍高速**（HTTP/2 の多重化が効果を発揮）

## なぜ gRPC の方が速いのか

1. **バイナリ vs テキスト**: Protocol Buffers はバイナリ形式なので、JSON よりシリアライズ/デシリアライズが高速かつデータサイズが小さい
2. **HTTP/2**: gRPC は HTTP/2 を使い、1つの接続上で複数リクエストを多重化できる。REST（HTTP/1.1）は基本的に1リクエストごとに接続のオーバーヘッドがある
3. **コード生成**: 型やメソッドが事前にコンパイルされているため、実行時のリフレクション（JSON の場合に必要）が不要
