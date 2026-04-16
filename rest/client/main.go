package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func doJSON(method, url string, body any, result any) *http.Response {
	var req *http.Request
	var err error

	if body != nil {
		b, _ := json.Marshal(body)
		req, err = http.NewRequest(method, url, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if result != nil {
		json.NewDecoder(resp.Body).Decode(result)
	}
	return resp
}

func main() {
	fmt.Println("=== REST クライアント: リソース（名詞）+ HTTPメソッドでAPIを呼び出す ===")
	fmt.Println()

	// 一覧取得
	fmt.Println("--- GET /users ---")
	var users []User
	doJSON("GET", "http://localhost:8080/users", nil, &users)
	for _, u := range users {
		fmt.Printf("  ID:%d Name:%s Email:%s Age:%d\n", u.ID, u.Name, u.Email, u.Age)
	}

	// 作成
	fmt.Println("\n--- POST /users  body: {name:Taro, email:taro@example.com, age:30} ---")
	var created User
	doJSON("POST", "http://localhost:8080/users", map[string]any{
		"name": "Taro", "email": "taro@example.com", "age": 30,
	}, &created)
	fmt.Printf("  作成: ID:%d Name:%s\n", created.ID, created.Name)

	// 取得
	fmt.Printf("\n--- GET /users/%d ---\n", created.ID)
	var got User
	doJSON("GET", fmt.Sprintf("http://localhost:8080/users/%d", created.ID), nil, &got)
	fmt.Printf("  取得: ID:%d Name:%s Email:%s Age:%d\n", got.ID, got.Name, got.Email, got.Age)

	// 更新
	fmt.Printf("\n--- PUT /users/%d  body: {name:Taro Yamada, age:31} ---\n", created.ID)
	var updated User
	doJSON("PUT", fmt.Sprintf("http://localhost:8080/users/%d", created.ID), map[string]any{
		"name": "Taro Yamada", "email": "taro@example.com", "age": 31,
	}, &updated)
	fmt.Printf("  更新: ID:%d Name:%s Age:%d\n", updated.ID, updated.Name, updated.Age)

	// 検索
	fmt.Println("\n--- GET /users?name=User ---")
	var results []User
	doJSON("GET", "http://localhost:8080/users?name=User", nil, &results)
	fmt.Printf("  検索結果: %d件\n", len(results))
	for _, u := range results {
		fmt.Printf("  ID:%d Name:%s\n", u.ID, u.Name)
	}

	// 削除
	fmt.Printf("\n--- DELETE /users/%d ---\n", created.ID)
	var delResp map[string]bool
	doJSON("DELETE", fmt.Sprintf("http://localhost:8080/users/%d", created.ID), nil, &delResp)
	fmt.Printf("  削除: success=%v\n", delResp["success"])

	// 削除後の一覧
	fmt.Println("\n--- GET /users ---")
	var remaining []User
	doJSON("GET", "http://localhost:8080/users", nil, &remaining)
	fmt.Printf("  残り: %d件\n", len(remaining))
}
