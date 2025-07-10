package template

import (
	"admin-dashboard/models"
	"html/template"
	"log"
	"net/http"
	"os"
)

// Global template variable accessible to other packages
var Templates *template.Template

// Add these variables
var globalTemplateData map[string]interface{}

// InitTemplates initializes all templates
func InitTemplates() {
	log.Printf("🚀 Initializing templates...")

	// Create base template with functions
	funcMap := template.FuncMap{
		"reverse": func(messages []models.Message) []models.Message {
			// Create a new slice with reversed order
			reversed := make([]models.Message, len(messages))
			for i, msg := range messages {
				reversed[len(messages)-1-i] = msg
			}
			return reversed
		},
	}

	t := template.New("").Funcs(funcMap)

	// Read message-bubble.html first
	messageBubbleContent, err := os.ReadFile("templates/components/message-bubble.html")
	if err != nil {
		log.Fatalf("❌ Could not read message-bubble.html: %v", err)
	}

	t, err = t.Parse(string(messageBubbleContent))
	if err != nil {
		log.Fatalf("❌ Could not parse message-bubble.html: %v", err)
	}

	// Parse remaining templates
	files := []string{
		"templates/layout.html",
		"templates/messages.html",
		"templates/login.html",
		"templates/components/chat-view.html",
		"templates/components/message-list.html",
		"templates/components/thread-preview.html",
		"templates/components/chat-messages.html",
	}

	Templates, err = t.ParseFiles(files...)
	if err != nil {
		log.Fatalf("❌ Could not parse templates: %v", err)
	}

	log.Printf("✅ Templates initialized successfully")
	log.Printf("📋 Available templates: %v", Templates.DefinedTemplates())
}

// Add this function
func SetGlobalTemplateData(data map[string]interface{}) {
	globalTemplateData = data
}

// Modify your RenderTemplate function
func RenderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	log.Printf("🎨 Rendering template: %s", name)

	// Merge global data with template-specific data
	var mergedData map[string]interface{}

	if data == nil {
		// If no data was provided, just use the global data
		mergedData = globalTemplateData
	} else {
		// If data was provided, merge it with the global data
		mergedData = make(map[string]interface{})

		// First, copy the global data
		for k, v := range globalTemplateData {
			mergedData[k] = v
		}

		// Then, add the template-specific data
		switch d := data.(type) {
		case map[string]interface{}:
			for k, v := range d {
				mergedData[k] = v
			}
		default:
			// If it's not a map, just use it directly
			return Templates.ExecuteTemplate(w, name, data)
		}
	}

	err := Templates.ExecuteTemplate(w, name, mergedData)
	if err != nil {
		log.Printf("❌ Error rendering template %s: %v", name, err)
	}
	return err
}
