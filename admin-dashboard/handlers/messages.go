package handlers

import (
    "log"
    "net/http"
    "html/template"
    "admin-dashboard/db"
    "admin-dashboard/models"
)

func GetMessages(w http.ResponseWriter, r *http.Request) {
    // Fetch messages from database
    rows, err := db.DB.Query(`
        WITH latest_messages AS (
            SELECT DISTINCT ON (thread_id) 
                id, client_id, page_id, platform, from_user,
                content, timestamp, thread_id, read
            FROM messages
            ORDER BY thread_id, timestamp DESC
        )
        SELECT * FROM latest_messages
        ORDER BY timestamp DESC
    `)
    if err != nil {
        log.Printf("Error fetching messages: %v", err)
        http.Error(w, "Error fetching messages", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var messages []models.Message
    for rows.Next() {
        var msg models.Message
        err := rows.Scan(
            &msg.ID, &msg.ClientID, &msg.PageID, &msg.Platform,
            &msg.FromUser, &msg.Content, &msg.Timestamp,
            &msg.ThreadID, &msg.Read,
        )
        if err != nil {
            log.Printf("Error scanning message: %v", err)
            continue
        }
        messages = append(messages, msg)
    }

    // Parse templates
    tmpl := template.New("")
    
    // Parse component templates first
    _, err = tmpl.ParseFiles(
        "templates/components/message-list.html",
        "templates/components/chat-view.html",
    )
    if err != nil {
        log.Printf("Error parsing components: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Then parse main templates
    _, err = tmpl.ParseFiles(
        "templates/layout.html",
        "templates/messages.html",
    )
    if err != nil {
        log.Printf("Error parsing main templates: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Pass the messages to the template
    data := map[string]interface{}{
        "Messages": messages,
    }

    // Execute template with data
    err = tmpl.ExecuteTemplate(w, "layout.html", data)
    if err != nil {
        log.Printf("Error executing template: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

func GetChat(w http.ResponseWriter, r *http.Request) {
    // Get the thread_id from the message that was clicked
    threadID := r.URL.Query().Get("thread_id")
    
    // Fetch messages for this thread
    rows, err := db.DB.Query(`
        SELECT id, client_id, page_id, platform, from_user, 
               content, timestamp, thread_id, read 
        FROM messages 
        WHERE thread_id = $1
        ORDER BY timestamp ASC
    `, threadID)
    if err != nil {
        log.Printf("Error fetching chat: %v", err)
        http.Error(w, "Error fetching chat", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var messages []models.Message
    for rows.Next() {
        var msg models.Message
        err := rows.Scan(
            &msg.ID, &msg.ClientID, &msg.PageID, &msg.Platform,
            &msg.FromUser, &msg.Content, &msg.Timestamp,
            &msg.ThreadID, &msg.Read,
        )
        if err != nil {
            log.Printf("Error scanning message: %v", err)
            continue
        }
        messages = append(messages, msg)
    }

    data := map[string]interface{}{
        "Messages": messages,
    }

    tmpl := template.Must(template.ParseFiles("templates/components/chat-view.html"))
    tmpl.ExecuteTemplate(w, "chat-view", data)
}

func SendMessage(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse the form data
    err := r.ParseForm()
    if err != nil {
        log.Printf("Error parsing form: %v", err)
        http.Error(w, "Error parsing form", http.StatusBadRequest)
        return
    }

    threadID := r.FormValue("thread_id")
    content := r.FormValue("message")

    // Insert the new message
    _, err = db.DB.Exec(`
        INSERT INTO messages (
            client_id,
            page_id,
            platform,
            from_user,
            content,
            thread_id,
            read
        ) SELECT 
            client_id,
            page_id,
            platform,
            'admin',
            $1,
            $2,
            true
        FROM messages 
        WHERE thread_id = $2 
        LIMIT 1
    `, content, threadID)

    if err != nil {
        log.Printf("Error inserting message: %v", err)
        http.Error(w, "Error sending message", http.StatusInternalServerError)
        return
    }

    // Instead of calling GetChat directly, set the thread_id in the URL query
    r.URL.RawQuery = "thread_id=" + threadID
    GetChat(w, r)
}

func GetMessageList(w http.ResponseWriter, r *http.Request) {
    // Fetch only the latest message from each thread
    rows, err := db.DB.Query(`
        WITH latest_messages AS (
            SELECT DISTINCT ON (thread_id) 
                id, client_id, page_id, platform, from_user,
                content, timestamp, thread_id, read
            FROM messages
            ORDER BY thread_id, timestamp DESC
        )
        SELECT * FROM latest_messages
        ORDER BY timestamp DESC
    `)
    if err != nil {
        log.Printf("Error fetching messages: %v", err)
        http.Error(w, "Error fetching messages", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var messages []models.Message
    for rows.Next() {
        var msg models.Message
        err := rows.Scan(
            &msg.ID, &msg.ClientID, &msg.PageID, &msg.Platform,
            &msg.FromUser, &msg.Content, &msg.Timestamp,
            &msg.ThreadID, &msg.Read,
        )
        if err != nil {
            log.Printf("Error scanning message: %v", err)
            continue
        }
        messages = append(messages, msg)
    }

    // Parse only the message-list template
    tmpl := template.Must(template.ParseFiles("templates/components/message-list.html"))
    tmpl.ExecuteTemplate(w, "message-list", map[string]interface{}{
        "Messages": messages,
    })
}