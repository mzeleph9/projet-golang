package main

import (
    "bufio"
    "fmt"
    "log"
    "os"
    "strings"
    "github.com/gorilla/websocket"
)

func main() {
    url := "ws://localhost:8000/ws"
    conn, _, err := websocket.DefaultDialer.Dial(url, nil)
    if err != nil {
        log.Fatal("Erreur de connexion au serveur WebSocket:", err)
    }
    defer conn.Close()

    fmt.Print("Entrez votre nom d'utilisateur : ")
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Scan()
    username := scanner.Text()

    fmt.Print("Entrez le nom de la room (room1, room2, room3...) : ")
    scanner.Scan()
    currentRoom := scanner.Text()

    roomChangeMsg := map[string]string{
        "type":     "room_change",
        "room":     currentRoom,
        "username": username,
    }
    err = conn.WriteJSON(roomChangeMsg)
    if err != nil {
        log.Fatal("Erreur lors du changement de room:", err)
    }

    // Goroutine pour recevoir les messages
    go func() {
        for {
            var msg Message
            err := conn.ReadJSON(&msg)
            if err != nil {
                log.Println("Erreur de lecture:", err)
                return
            }

            // Formater différemment selon le type de message
            switch msg.Type {
            case "system":
                fmt.Printf("\n[System] %s\n", msg.Message)
            case "message":
                fmt.Printf("\n[%s] %s: %s\n", msg.Room, msg.Username, msg.Message)
            }
            fmt.Print("> ")
        }
    }()

    for {
        fmt.Print("> ")
        scanner.Scan()
        input := scanner.Text()

        if strings.HasPrefix(input, "/join ") {
            newRoom := strings.TrimPrefix(input, "/join ")
            currentRoom = newRoom
            roomChangeMsg := map[string]string{
                "type":     "room_change",
                "room":     newRoom,
                "username": username,
            }
            err = conn.WriteJSON(roomChangeMsg)
            if err != nil {
                log.Println("Erreur lors du changement de room:", err)
            }
            fmt.Printf("Vous avez rejoint la room: %s\n", newRoom)
            continue
        }

        message := map[string]string{
            "type":     "message",
            "room":     currentRoom,
            "username": username,
            "message":  input,
        }

        err = conn.WriteJSON(message)
        if err != nil {
            log.Println("Erreur lors de l'envoi du message:", err)
        }
    }
}

type Message struct {
    Type     string `json:"type"`
    Room     string `json:"room"`
    Username string `json:"username"`
    Message  string `json:"message"`
}