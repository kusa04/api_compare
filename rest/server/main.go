package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

type store struct {
	mu     sync.RWMutex
	users  map[int]*User
	nextID int
}

var db *store

func init() {
	db = &store{users: make(map[int]*User), nextID: 1}
	for i := 0; i < 100; i++ {
		db.users[db.nextID] = &User{
			ID:    db.nextID,
			Name:  fmt.Sprintf("User %d", db.nextID),
			Email: fmt.Sprintf("user%d@example.com", db.nextID),
			Age:   20 + i*5,
		}
		db.nextID++
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// GET /users       → 一覧 (検索: ?name=xxx, 件数制限: ?limit=N)
// POST /users      → 作成
func usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		db.mu.RLock()
		defer db.mu.RUnlock()

		nameQuery := r.URL.Query().Get("name")
		results := make([]User, 0, len(db.users))
		for _, u := range db.users {
			if nameQuery == "" || strings.Contains(strings.ToLower(u.Name), strings.ToLower(nameQuery)) {
				results = append(results, *u)
			}
		}
		// ID 昇順で安定したレスポンスを返す
		sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })

		// ?limit=N が指定され、正の値かつ全件数より小さければ先頭 N 件に絞る
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(results) {
				results = results[:limit]
			}
		}
		writeJSON(w, http.StatusOK, results)

	case http.MethodPost:
		var input struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Age   int    `json:"age"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		db.mu.Lock()
		user := &User{ID: db.nextID, Name: input.Name, Email: input.Email, Age: input.Age}
		db.users[db.nextID] = user
		db.nextID++
		db.mu.Unlock()
		writeJSON(w, http.StatusCreated, user)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /users/{id}    → 取得
// PUT /users/{id}    → 更新
// DELETE /users/{id} → 削除
func userHandler(w http.ResponseWriter, r *http.Request) {
	// パスから ID を取得: /users/123 → "123"
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(parts[2])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		db.mu.RLock()
		user, ok := db.users[id]
		db.mu.RUnlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, user)

	case http.MethodPut:
		var input struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Age   int    `json:"age"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		db.mu.Lock()
		user, ok := db.users[id]
		if !ok {
			db.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		user.Name = input.Name
		user.Email = input.Email
		user.Age = input.Age
		db.mu.Unlock()
		writeJSON(w, http.StatusOK, user)

	case http.MethodDelete:
		db.mu.Lock()
		if _, ok := db.users[id]; !ok {
			db.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		delete(db.users, id)
		db.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	// REST のルーティング: リソース（名詞） + HTTPメソッド（動詞）
	//   GET    /users        → 一覧・検索（?name=, ?limit=）
	//   POST   /users        → 作成
	//   GET    /users/{id}   → 取得
	//   PUT    /users/{id}   → 更新
	//   DELETE /users/{id}   → 削除
	http.HandleFunc("/users", usersHandler) // コレクション
	http.HandleFunc("/users/", userHandler) // 個別リソース

	fmt.Println("REST server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
