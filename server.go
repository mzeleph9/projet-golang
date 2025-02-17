package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "github.com/go-redis/redis/v8"
    "github.com/gorilla/websocket"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

var rdb *redis.Client
var db *sqlx.DB
var clients = make(map[*websocket.Conn]string)

func main() {
    // Connexion à Redis
    rdb = redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", 
        DB:       0,
    })

    // Connexion à PostgreSQL
    var err error
    db, err = sqlx.Connect("postgres", "user=mennan dbname=chatdb sslmode=disable password=mennan")
    if err != nil {
        log.Fatalln(err)
    }

    // Création de la table messages si elle n'existe pas
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS messages (
            id SERIAL PRIMARY KEY,
            room VARCHAR(50) NOT NULL,
            username VARCHAR(50) NOT NULL,
            message TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
    if err != nil {
        log.Fatalln("Erreur création table:", err)
    }

    http.HandleFunc("/ws", handleConnections)
    go handleMessages()
    
    log.Println("http server started on :8000")
    err = http.ListenAndServe(":8000", nil)
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer ws.Close()

    clients[ws] = ""
    defer delete(clients, ws)

    for {
        var msg Message
        err := ws.ReadJSON(&msg)
        if err != nil {
            log.Printf("error: %v", err)
            break
        }

        if msg.Type == "room_change" {
            oldRoom := clients[ws]
            clients[ws] = msg.Room
            log.Printf("Client switched from room %s to room %s", oldRoom, msg.Room)
            
            // Envoyer un message système à la nouvelle room
            systemMsg := Message{
                Type:     "system",
                Room:     msg.Room,
                Username: "System",
                Message:  fmt.Sprintf("%s a rejoint la room", msg.Username),
            }
            
            // Envoyer aux clients de la nouvelle room
            for client, clientRoom := range clients {
                if clientRoom == msg.Room {
                    err := client.WriteJSON(systemMsg)
                    if err != nil {
                        log.Printf("error sending message to client: %v", err)
                        client.Close()
                        delete(clients, client)
                    }
                }
            }
            continue
        }

        log.Printf("[%s] %s: %s", msg.Room, msg.Username, msg.Message)

        // Sauvegarder le message dans PostgreSQL
        _, err = db.Exec("INSERT INTO messages (room, username, message) VALUES ($1, $2, $3)", 
            msg.Room, msg.Username, msg.Message)
        if err != nil {
            log.Printf("error saving message: %v", err)
        }

        // Envoyer le message aux clients dans la même room
        for client, clientRoom := range clients {
            if clientRoom == msg.Room {
                err := client.WriteJSON(msg)
                if err != nil {
                    log.Printf("error sending message to client: %v", err)
                    client.Close()
                    delete(clients, client)
                }
            }
        }
    }
}

func handleMessages() {
    pubsub := rdb.Subscribe(context.Background(), "room1", "room2", "room3")
    defer pubsub.Close()

    ch := pubsub.Channel()
    for msg := range ch {
        fmt.Println(msg.Channel, msg.Payload)
        
        for client, clientRoom := range clients {
            if clientRoom == msg.Channel {
                err := client.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
                if err != nil {
                    log.Printf("error sending message to client: %v", err)
                    client.Close()
                    delete(clients, client)
                }
            }
        }
    }
}

type Message struct {
    Type     string `json:"type"`     // "message", "room_change", ou "system"
    Room     string `json:"room"`
    Username string `json:"username"`
    Message  string `json:"message"`
}