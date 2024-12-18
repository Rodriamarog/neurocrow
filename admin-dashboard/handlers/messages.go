package handlers

import (
    "log"
    "net/http"
    "html/template"
    "strings"
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

    // Create a new template with all required files
    tmpl, err := template.ParseFiles(
        "templates/layout.html",
        "templates/messages.html",
        "templates/components/message-list.html",
        "templates/components/chat-view.html",
    )
    if err != nil {
        log.Printf("Error parsing templates: %v", err)
        http.Error(w, "Error loading templates", http.StatusInternalServerError)
        return
    }

    // Pass the messages to the template
    data := map[string]interface{}{
        "Messages": messages,
    }

    // Execute template with data, ignore broken pipe errors
    if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
        if !strings.Contains(err.Error(), "write: broken pipe") {
            log.Printf("Error executing template: %v", err)
        }
        return
    }
}

func GetChat(w http.ResponseWriter, r *http.Request) {
    // Get the thread_id from the message that was clicked
    threadID := r.URL.Query().Get("thread_id")
    log.Printf("GetChat called with thread_id: %s", threadID)
    
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

    log.Printf("Found %d messages for thread %s", len(messages), threadID)

    data := map[string]interface{}{
        "Messages": messages,
    }

    tmpl := template.Must(template.ParseFiles("templates/components/chat-view.html"))
    err = tmpl.ExecuteTemplate(w, "chat-view", data)
    if err != nil {
        log.Printf("Error executing template: %v", err)
        http.Error(w, "Error rendering chat", http.StatusInternalServerError)
        return
    }
    log.Printf("Successfully rendered chat view for thread %s", threadID)
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
    content := strings.TrimSpace(r.FormValue("message"))

    // Prevent empty messages
    if content == "" {
        http.Error(w, "Empty message", http.StatusBadRequest)
        return
    }

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

    // After successfully sending the message, return both the message and trigger preview update
    tmpl := template.Must(template.New("message-response").Parse(`
        <div class="flex items-start max-w-[85%] justify-end ml-auto"
             hx-get="/thread-preview?thread_id={{.ThreadID}}"
             hx-target="#thread-preview-{{.ThreadID}}"
             hx-trigger="load"
             hx-swap="outerHTML">
            <div class="bg-indigo-600 text-white rounded-lg px-4 py-2">
                <p class="text-sm">{{.Content}}</p>
            </div>
        </div>
    `))

    err = tmpl.Execute(w, struct {
        ThreadID string
        Content  string
    }{
        ThreadID: threadID,
        Content:  content,
    })
    if err != nil {
        log.Printf("Error executing template: %v", err)
        http.Error(w, "Error rendering message", http.StatusInternalServerError)
        return
    }
}

func GetMessageList(w http.ResponseWriter, r *http.Request) {
    rows, err := db.DB.Query(`
        WITH thread_owner AS (
            -- Get first message of each thread
            SELECT DISTINCT ON (thread_id)
                thread_id, 
                from_user as original_sender
            FROM messages
            ORDER BY thread_id, timestamp ASC
        ),
        latest_messages AS (
            -- Get latest message of each thread
            SELECT DISTINCT ON (thread_id)
                m.*, 
                t.original_sender as thread_owner
            FROM messages m
            JOIN thread_owner t ON m.thread_id = t.thread_id
            ORDER BY thread_id, timestamp DESC
        )
        SELECT 
            id, client_id, page_id, platform,
            thread_owner as from_user,  
            content, timestamp, thread_id, read
        FROM latest_messages
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

    // If this is an HTMX request, just return the message list partial
    if r.Header.Get("HX-Request") == "true" {
        tmpl := template.Must(template.ParseFiles("templates/components/message-list.html"))
        tmpl.ExecuteTemplate(w, "message-list", map[string]interface{}{
            "Messages": messages,
        })
        return
    }

    // Otherwise return the full template
    tmpl := template.Must(template.ParseFiles("templates/components/message-list.html"))
    tmpl.ExecuteTemplate(w, "message-list", map[string]interface{}{
        "Messages": messages,
    })
}

func GetThreadPreview(w http.ResponseWriter, r *http.Request) {
    threadID := r.URL.Query().Get("thread_id")
    
    // Query for just this thread's latest message
    row := db.DB.QueryRow(`
        WITH thread_owner AS (
            SELECT DISTINCT ON (thread_id)
                thread_id, 
                from_user as original_sender
            FROM messages
            WHERE thread_id = $1
            ORDER BY thread_id, timestamp ASC
        )
        SELECT 
            m.id, m.client_id, m.page_id, m.platform,
            t.original_sender as from_user,
            m.content, m.timestamp, m.thread_id, m.read
        FROM messages m
        JOIN thread_owner t ON m.thread_id = t.thread_id
        WHERE m.thread_id = $1
        ORDER BY m.timestamp DESC
        LIMIT 1
    `, threadID)

    var msg models.Message
    err := row.Scan(
        &msg.ID, &msg.ClientID, &msg.PageID, &msg.Platform,
        &msg.FromUser, &msg.Content, &msg.Timestamp,
        &msg.ThreadID, &msg.Read,
    )
    if err != nil {
        log.Printf("Error scanning message: %v", err)
        http.Error(w, "Error fetching thread preview", http.StatusInternalServerError)
        return
    }

    tmpl := template.Must(template.New("thread-preview").Parse(`
    <div class="p-4 hover:bg-gray-50 active:bg-gray-100 cursor-pointer"
         id="thread-preview-{{.ThreadID}}"
         hx-get="/chat?thread_id={{.ThreadID}}"
         hx-target="#chat-view"
         hx-trigger="click"
         _="on htmx:afterOnLoad remove .hidden from #chat-view then remove .translate-x-full from #chat-view">
        <div class="flex items-center justify-between mb-1">
            <div class="flex items-center">
                <div class="w-2 h-2 {{if eq .Platform "facebook"}}bg-blue-500{{else}}bg-pink-500{{end}} rounded-full mr-2"></div>
                <span class="text-sm font-medium {{if eq .Platform "facebook"}}text-blue-600{{else}}text-pink-600{{end}}">
                    {{if eq .Platform "facebook"}}Facebook{{else}}Instagram{{end}}
                </span>
            </div>
            <span class="text-xs text-gray-500">{{.Timestamp.Format "15:04"}}</span>
        </div>
        <div class="flex items-center">
            <div class="h-12 w-12 rounded-full bg-gray-200"></div>
            <div class="ml-3 flex-1">
                <div class="text-sm font-medium text-gray-900">{{.FromUser}}</div>
                <div class="text-sm text-gray-500 truncate">{{.Content}}</div>
            </div>
        </div>
    </div>
    `))

    tmpl.Execute(w, msg)
}