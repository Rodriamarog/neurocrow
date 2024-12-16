package main

import (
    "log"
    "net/http"
    "os"
    "admin-dashboard/db"
    "admin-dashboard/handlers"
)

func main() {
    // Initialize database
    db.Init()
    defer db.DB.Close()

    // Routes
    http.HandleFunc("/", handlers.GetMessages)
    http.HandleFunc("/messages", handlers.GetMessageList)  // Add this line
    http.HandleFunc("/chat", handlers.GetChat)
    http.HandleFunc("/send-message", handlers.SendMessage)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    log.Printf("Server starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}