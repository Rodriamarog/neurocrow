// message-router/sentiment/sentiment_test.go
package sentiment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	// Try to load .env from the message-router directory
	currentDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Look for .env in current directory and parent directory
	envPaths := []string{
		filepath.Join(currentDir, ".env"),
		filepath.Join(filepath.Dir(currentDir), ".env"),
	}

	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}
}

func TestSentimentAnalyzer(t *testing.T) {
	// Get API key from environment
	apiKey := os.Getenv("FIREWORKS_API_KEY")
	if apiKey == "" {
		t.Fatal("FIREWORKS_API_KEY not set in .env file")
	}

	// Create config
	config := DefaultConfig()
	config.FireworksKey = apiKey

	// Create analyzer
	analyzer := New(config)

	// Test cases
	tests := []struct {
		name    string
		message string
		want    string // Expected status
		wantErr bool
	}{
		{
			name:    "General inquiry",
			message: "¿Cuál es el horario de atención?",
			want:    "general",
		},
		{
			name:    "Human request",
			message: "Necesito hablar con una persona por favor",
			want:    "need_human",
		},
		{
			name:    "Frustrated user",
			message: "Ya te dije tres veces que ese no es mi problema! No me estás entendiendo!",
			want:    "frustrated",
		},
		{
			name:    "Multiple human requests",
			message: "Por favor conectame con un agente. NECESITO HABLAR CON ALGUIEN YA!",
			want:    "frustrated", // Prioritizes frustration over human request
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add delay between tests to respect rate limits
			time.Sleep(500 * time.Millisecond)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			t.Logf("Testing message: %q", tt.message)
			analysis, err := analyzer.Analyze(ctx, tt.message)

			if (err != nil) != tt.wantErr {
				t.Errorf("Analyze() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && analysis.Status != tt.want {
				t.Errorf("Analyze() got status = %v, want %v", analysis.Status, tt.want)
			}

			if analysis != nil {
				t.Logf("Result: status=%s confidence=%.2f tokens=%d (≈$%.5f)",
					analysis.Status,
					analysis.Confidence,
					analysis.TokensUsed,
					float64(analysis.TokensUsed)*0.20/1_000_000) // $0.20 per 1M tokens
			}
		})
	}
}

// TestCustomMessages allows manual testing of specific messages
func TestCustomMessages(t *testing.T) {
	apiKey := os.Getenv("FIREWORKS_API_KEY")
	if apiKey == "" {
		t.Fatal("FIREWORKS_API_KEY not set in .env file")
	}

	config := DefaultConfig()
	config.FireworksKey = apiKey
	analyzer := New(config)

	messages := []string{
		"No me estás entendiendo, esto es inútil",
		"Quiero cambiar mi plan",
		"CONECTAME CON UN HUMANO YA!!!!",
		"La verdad que este bot no sirve para nada",
		"¿Pueden decirme cuánto cuesta la suscripción?",
	}

	for _, msg := range messages {
		// Add delay between tests to respect rate limits
		time.Sleep(500 * time.Millisecond)

		t.Logf("\nTesting message: %q", msg)

		analysis, err := analyzer.Analyze(context.Background(), msg)
		if err != nil {
			t.Errorf("Error analyzing message: %v", err)
			continue
		}

		t.Logf("Result: status=%s confidence=%.2f", analysis.Status, analysis.Confidence)
	}
}

// TestGeneralCategorization tests ~100 Spanish phrases that should be categorized as 'general'
func TestGeneralCategorization(t *testing.T) {
	apiKey := os.Getenv("FIREWORKS_API_KEY")
	if apiKey == "" {
		t.Fatal("FIREWORKS_API_KEY not set in .env file")
	}

	config := DefaultConfig()
	config.FireworksKey = apiKey
	analyzer := New(config)

	// List of Spanish phrases that should be categorized as 'general'
	generalPhrases := []string{
		// Basic greetings and questions
		"Hola, ¿cómo estás?",
		"Buenos días",
		"Buenas tardes",
		"¿Qué tal?",
		"¿Cómo te va?",
		"Saludos",
		"Muy buenas",
		"¿Qué hay de nuevo?",
		"¿Cómo van las cosas?",
		"Hola, ¿qué tal todo?",

		// Information requests
		"¿Cuál es el horario de atención?",
		"¿A qué hora abren?",
		"¿Cuándo cierran?",
		"¿Dónde están ubicados?",
		"¿Cuál es la dirección?",
		"¿Tienen WhatsApp?",
		"¿Cuál es el teléfono?",
		"¿Cómo los puedo contactar?",
		"¿Tienen redes sociales?",
		"¿Cuál es su página web?",

		// Product/service inquiries
		"¿Qué servicios ofrecen?",
		"¿Cuáles son sus productos?",
		"¿Tienen descuentos?",
		"¿Cuáles son los precios?",
		"¿Hay promociones?",
		"¿Tienen stock disponible?",
		"¿Cuánto cuesta esto?",
		"¿Qué incluye el servicio?",
		"¿Hay garantía?",
		"¿Hacen entregas a domicilio?",

		// Availability and scheduling
		"¿Tienen disponibilidad para mañana?",
		"¿Puedo agendar una cita?",
		"¿Qué días trabajan?",
		"¿Atienden los fines de semana?",
		"¿Hay espacio para hoy?",
		"¿Cuándo puedo ir?",
		"¿Necesito reservar?",
		"¿Puedo pasar sin cita?",
		"¿Qué horarios manejan?",
		"¿Están abiertos ahora?",

		// Process and procedures
		"¿Cómo funciona el proceso?",
		"¿Qué documentos necesito?",
		"¿Cuáles son los requisitos?",
		"¿Qué pasos debo seguir?",
		"¿Cómo hago el pago?",
		"¿Aceptan tarjetas?",
		"¿Puedo pagar en efectivo?",
		"¿Hay que hacer depósito?",
		"¿Cuánto tiempo toma?",
		"¿Cuál es el procedimiento?",

		// General requests
		"Me gustaría saber más información",
		"¿Pueden enviarme detalles?",
		"Necesito más datos",
		"¿Me pueden explicar mejor?",
		"Quiero conocer las opciones",
		"¿Qué me recomiendan?",
		"Estoy interesado en sus servicios",
		"Me gusta lo que ofrecen",
		"¿Cómo puedo empezar?",
		"Quiero contratar",

		// Polite expressions
		"Muchas gracias",
		"Se lo agradezco",
		"Muy amable",
		"Perfecto, gracias",
		"Excelente servicio",
		"Están muy atentos",
		"Me han ayudado mucho",
		"Todo muy claro",
		"Entendido, gracias",
		"Muy buena atención",

		// Simple statements
		"Me interesa",
		"Está bien",
		"Perfecto",
		"De acuerdo",
		"Entiendo",
		"Ok, gracias",
		"Muy bien",
		"Claro",
		"Por supuesto",
		"Sin problema",

		// Casual inquiries
		"¿Qué me cuentan?",
		"¿Cómo va todo?",
		"¿Todo bien por ahí?",
		"¿Qué novedades hay?",
		"¿Hay algo nuevo?",
		"¿Cómo están?",
		"¿Todo en orden?",
		"¿Qué tal el negocio?",
		"¿Cómo les va?",
		"¿Todo funcionando bien?",

		// Specific service questions
		"¿Hacen reparaciones?",
		"¿Tienen servicio técnico?",
		"¿Ofrecen instalación?",
		"¿Hay servicio post-venta?",
		"¿Tienen mantenimiento?",
		"¿Hacen revisiones?",
		"¿Ofrecen consultoría?",
		"¿Tienen capacitación?",
		"¿Hay soporte técnico?",
		"¿Ofrecen asesoría?",

		// Timing and duration
		"¿Cuánto demora?",
		"¿Qué tan rápido es?",
		"¿En cuánto tiempo está listo?",
		"¿Para cuándo estaría?",
		"¿Cuál es el tiempo de entrega?",
		"¿Cuándo puedo recoger?",
		"¿Qué tan pronto pueden?",
		"¿Es inmediato?",
		"¿Hay que esperar mucho?",
		"¿En qué plazo se hace?",

		// Final phrases to reach ~100
		"¿Tienen experiencia en esto?",
		"¿Son especialistas?",
		"¿Qué tan buenos son?",
		"¿Tienen referencias?",
		"¿Llevan mucho tiempo?",
		"¿Son de confianza?",
		"¿Qué opinan sus clientes?",
		"¿Tienen buenas reseñas?",
		"¿Son reconocidos?",
		"¿Qué los diferencia?",
	}

	var correctCount int
	var incorrectCount int
	var totalCost float64

	t.Logf("Testing %d Spanish phrases expected to be categorized as 'general'", len(generalPhrases))

	for i, phrase := range generalPhrases {
		// Add delay between tests to respect rate limits
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		
		analysis, err := analyzer.Analyze(ctx, phrase)
		cancel()

		if err != nil {
			t.Errorf("Error analyzing phrase %d '%s': %v", i+1, phrase, err)
			incorrectCount++
			continue
		}

		// Calculate cost (approximate)
		cost := float64(analysis.TokensUsed) * 0.20 / 1_000_000
		totalCost += cost

		if analysis.Status == "general" {
			correctCount++
			t.Logf("✅ [%d/%d] CORRECT: '%s' → %s (tokens: %d, ~$%.6f)", 
				i+1, len(generalPhrases), phrase, analysis.Status, analysis.TokensUsed, cost)
		} else {
			incorrectCount++
			t.Logf("❌ [%d/%d] INCORRECT: '%s' → %s (expected: general) (tokens: %d, ~$%.6f)", 
				i+1, len(generalPhrases), phrase, analysis.Status, analysis.TokensUsed, cost)
		}
	}

	// Summary
	accuracy := float64(correctCount) / float64(len(generalPhrases)) * 100
	t.Logf("\n" + strings.Repeat("=", 80))
	t.Logf("SUMMARY:")
	t.Logf("Total phrases tested: %d", len(generalPhrases))
	t.Logf("Correct categorizations: %d", correctCount)
	t.Logf("Incorrect categorizations: %d", incorrectCount)
	t.Logf("Accuracy: %.2f%%", accuracy)
	t.Logf("Total estimated cost: ~$%.6f", totalCost)
	t.Logf(strings.Repeat("=", 80))

	// Fail test if accuracy is below reasonable threshold
	if accuracy < 85.0 {
		t.Errorf("Accuracy %.2f%% is below acceptable threshold of 85%%", accuracy)
	}
}

// TestNeedHumanCategorization tests Spanish phrases that should be categorized as 'need_human'
func TestNeedHumanCategorization(t *testing.T) {
	apiKey := os.Getenv("FIREWORKS_API_KEY")
	if apiKey == "" {
		t.Fatal("FIREWORKS_API_KEY not set in .env file")
	}

	config := DefaultConfig()
	config.FireworksKey = apiKey
	analyzer := New(config)

	// List of Spanish phrases that should be categorized as 'need_human'
	needHumanPhrases := []string{
		// Direct requests for human help
		"Necesito hablar con una persona",
		"Quiero hablar con un humano",
		"¿Puedo hablar con alguien?",
		"Necesito hablar con un agente",
		"Quiero hablar con un representante",
		"¿Me pueden conectar con una persona?",
		"Necesito ayuda de un humano",
		"Quiero hablar con alguien real",
		"¿Hay alguna persona disponible?",
		"Necesito hablar con un operador",
		
		// Explicit agent requests
		"Conectame con un agente",
		"Transfiéreme a un agente",
		"Quiero un agente humano",
		"¿Puedo hablar con un agente?",
		"Necesito un agente por favor",
		"Ponme con un agente",
		"Quiero que me atienda una persona",
		"Transfiéreme con alguien",
		"¿Me puedes pasar con un agente?",
		"Solicito hablar con un agente",
		
		// Support/representative requests
		"Quiero hablar con soporte",
		"Necesito soporte humano",
		"¿Puedo hablar con atención al cliente?",
		"Quiero hablar con un representante de ventas",
		"Necesito hablar con un supervisor",
		"¿Me pueden pasar con soporte técnico?",
		"Quiero hablar con el departamento de ventas",
		"Necesito hablar con un especialista",
		"¿Hay algún humano que me pueda ayudar?",
		"Quiero hablar con alguien del equipo",
		
		// Bot rejection + human request
		"Este bot no me sirve, quiero una persona",
		"No entiendo al bot, necesito un humano",
		"El bot no me ayuda, quiero un agente",
		"Prefiero hablar con una persona real",
		"No me gusta hablar con bots, quiero un humano",
		"Este chatbot no funciona, necesito una persona",
		"No confío en los bots, quiero un agente",
		"Los bots son inútiles, necesito un humano",
		"No quiero bot, quiero persona",
		"Mejor ponme con alguien real",
		
		// Urgent human requests
		"NECESITO HABLAR CON UNA PERSONA YA",
		"URGENTE: quiero un agente",
		"Es urgente, necesito un humano",
		"¡Ponme con alguien ahora mismo!",
		"AYUDA! Necesito una persona",
		"Emergencia, quiero un agente",
		"¡YA! Necesito hablar con alguien",
		"Rápido, conectame con una persona",
		"¡AHORA! Quiero un humano",
		"¡Por favor! Necesito un agente real",
		
		// Formal human requests
		"Solicito ser atendido por una persona",
		"Requiero asistencia humana",
		"Deseo hablar con un representante",
		"Me gustaría hablar con alguien del equipo",
		"Quisiera ser atendido por una persona",
		"Solicito hablar con atención al cliente",
		"Requiero asistencia de un agente",
		"Deseo contactar con un especialista",
		"Me gustaría hablar con un supervisor",
		"Quisiera ser transferido a un agente",
		
		// Multiple ways to ask for human
		"¿Hay alguna persona que me pueda atender?",
		"¿Me pueden conectar con alguien real?",
		"¿Existe la opción de hablar con un humano?",
		"¿Puedo ser atendido por una persona?",
		"¿Es posible hablar con un agente?",
		"¿Me puedes pasar con alguien del equipo?",
		"¿Hay alguien disponible para atenderme?",
		"¿Puedo solicitar atención humana?",
		"¿Me pueden dar asistencia humana?",
		"¿Hay opción de hablar con una persona?",
		
		// Variations with 'persona'
		"Quiero una persona",
		"Necesito una persona",
		"Busco una persona",
		"Prefiero una persona",
		"Solicito una persona",
		"Requiero una persona",
		"Dame una persona",
		"Ponme una persona",
		"Consígueme una persona",
		"Tráeme una persona",
		
		// Service-specific human requests
		"Quiero hablar con ventas humano",
		"Necesito soporte técnico humano",
		"Quiero atención al cliente real",
		"Necesito un vendedor humano",
		"Quiero un técnico real",
		"Necesito gerente humano",
		"Quiero supervisor real",
		"Necesito ejecutivo humano",
		"Quiero asesor real",
		"Necesito consultor humano",
		
		// Polite but clear human requests
		"Por favor, me gustaría hablar con una persona",
		"Si es posible, quisiera un agente humano",
		"¿Sería posible hablar con alguien?",
		"Por favor conectame con una persona",
		"Si puedes, ponme con un humano",
		"Te agradecería hablar con un agente",
		"Por favor, necesito una persona",
		"Si está disponible, quiero un humano",
		"Por favor, un representante humano",
		"Si es posible, un agente real",
		
		// Direct 'humano' requests
		"Quiero un humano",
		"Necesito un humano",
		"Dame un humano",
		"Ponme un humano",
		"Busco un humano",
		"Requiero un humano",
		"Solicito un humano",
		"Prefiero un humano",
		"Consígueme un humano",
		"Tráeme un humano",
	}

	var correctCount int
	var incorrectCount int
	var totalCost float64

	t.Logf("Testing %d Spanish phrases expected to be categorized as 'need_human'", len(needHumanPhrases))

	for i, phrase := range needHumanPhrases {
		// Add delay between tests to respect rate limits
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		
		analysis, err := analyzer.Analyze(ctx, phrase)
		cancel()

		if err != nil {
			t.Errorf("Error analyzing phrase %d '%s': %v", i+1, phrase, err)
			incorrectCount++
			continue
		}

		// Calculate cost (approximate)
		cost := float64(analysis.TokensUsed) * 0.20 / 1_000_000
		totalCost += cost

		if analysis.Status == "need_human" {
			correctCount++
			t.Logf("✅ [%d/%d] CORRECT: '%s' → %s (tokens: %d, ~$%.6f)", 
				i+1, len(needHumanPhrases), phrase, analysis.Status, analysis.TokensUsed, cost)
		} else {
			incorrectCount++
			t.Logf("❌ [%d/%d] INCORRECT: '%s' → %s (expected: need_human) (tokens: %d, ~$%.6f)", 
				i+1, len(needHumanPhrases), phrase, analysis.Status, analysis.TokensUsed, cost)
		}
	}

	// Summary
	accuracy := float64(correctCount) / float64(len(needHumanPhrases)) * 100
	t.Logf("\n" + strings.Repeat("=", 80))
	t.Logf("NEED_HUMAN CATEGORIZATION SUMMARY:")
	t.Logf("Total phrases tested: %d", len(needHumanPhrases))
	t.Logf("Correct categorizations: %d", correctCount)
	t.Logf("Incorrect categorizations: %d", incorrectCount)
	t.Logf("Accuracy: %.2f%%", accuracy)
	t.Logf("Total estimated cost: ~$%.6f", totalCost)
	t.Logf(strings.Repeat("=", 80))

	// Fail test if accuracy is below reasonable threshold
	if accuracy < 85.0 {
		t.Errorf("Accuracy %.2f%% is below acceptable threshold of 85%%", accuracy)
	}
}
