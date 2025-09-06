// oauth/webhooks.go
// Webhook subscription and handover protocol functions - EXACT COPY from client-manager

package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// =============================================================================
// COPY THE FOLLOWING FUNCTIONS EXACTLY FROM CLIENT-MANAGER/MAIN.GO:
// =============================================================================
//
// 1. setupWebhookSubscriptions function (lines 1242-1277)
//    - Copy exactly as-is
//
// 2. subscribePageToWebhooks function (lines 1014-1099)
//    - Copy exactly as-is
//
// 3. configureHandoverProtocol function (lines 1102-1177)
//    - Copy exactly as-is
//
// 4. verifyWebhookSetup function (lines 1184-1241, if exists)
//    - Copy exactly as-is
//
// =============================================================================

func setupWebhookSubscriptions(pageID, pageToken, pageName, platform string) error {
	log.Printf("🚀 Starting webhook setup for %s page: %s (%s)", platform, pageName, pageID)

	if platform == "instagram" {
		// Instagram webhooks are configured at app level in Facebook App Dashboard
		// No per-page API subscription needed - webhooks work automatically once configured in dashboard
		log.Printf("📱 Instagram webhooks configured at app level - no API subscription needed")
		log.Printf("ℹ️ Instagram account %s will receive webhooks via app-level configuration", pageName)
		log.Printf("✅ Instagram webhook setup completed for %s (app-level configuration)", pageName)
		return nil
	}

	// Facebook pages require individual API subscriptions
	log.Printf("📘 Facebook page requires individual API webhook subscription")

	// Step 1: Subscribe page to webhooks (Facebook only)
	if err := subscribePageToWebhooks(pageID, pageToken, platform); err != nil {
		log.Printf("❌ Webhook subscription failed for %s: %v", pageName, err)
		return fmt.Errorf("webhook subscription failed: %v", err)
	}

	// REMOVED: Step 2: Configure handover protocol (removed as requested)
	// REMOVED: Step 3: Verify the setup (removed for simplicity)

	log.Printf("✅ Facebook webhook setup completed for %s page: %s", platform, pageName)
	return nil
}

func subscribePageToWebhooks(pageID, pageToken, platform string) error {
	appID := os.Getenv("FACEBOOK_APP_ID")
	if appID == "" {
		return fmt.Errorf("FACEBOOK_APP_ID environment variable not set")
	}

	// Subscribe page to the Neurocrow app for webhook events
	subscribeURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s/subscribed_apps", pageID)

	// Create platform-specific payload for subscribing to webhooks
	var subscribedFields []string

	if platform == "instagram" {
		// Instagram only supports basic messaging fields
		subscribedFields = []string{
			"messages",
			"messaging_postbacks",
		}
		log.Printf("📱 Using Instagram-specific webhook fields (messages, messaging_postbacks only)")
	} else {
		// Facebook pages support basic fields (REMOVED: messaging_handovers as requested)
		subscribedFields = []string{
			"messages",
			"messaging_postbacks",
			"messaging_policy_enforcement",
			"message_echoes",
		}
		log.Printf("📘 Using Facebook-specific webhook fields (removed messaging_handovers)")
	}

	subscribePayload := map[string]interface{}{
		"subscribed_fields": subscribedFields,
	}

	jsonData, err := json.Marshal(subscribePayload)
	if err != nil {
		return fmt.Errorf("error marshaling subscribe payload: %v", err)
	}

	// Make POST request to subscribe
	req, err := http.NewRequest("POST", subscribeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating subscribe request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.URL.RawQuery = fmt.Sprintf("access_token=%s", pageToken)

	log.Printf("🔗 Subscribing %s page %s to webhooks: %s", platform, pageID, subscribeURL)
	log.Printf("📤 Subscribe payload: %s", string(jsonData))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making subscribe request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading subscribe response: %v", err)
	}

	log.Printf("📥 Webhook subscription response: %d - %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		var fbError struct {
			Error struct {
				Message   string `json:"message"`
				Type      string `json:"type"`
				Code      int    `json:"code"`
				FbtraceID string `json:"fbtrace_id"`
			} `json:"error"`
		}

		if json.Unmarshal(respBody, &fbError) == nil && fbError.Error.Message != "" {
			return fmt.Errorf("Facebook webhook subscription error: %s (Code: %d, Trace: %s)",
				fbError.Error.Message, fbError.Error.Code, fbError.Error.FbtraceID)
		}
		return fmt.Errorf("webhook subscription failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("✅ Successfully subscribed %s page %s to webhooks", platform, pageID)
	return nil
}
