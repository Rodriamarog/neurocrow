package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// FacebookDebugger helps diagnose Facebook API issues
type FacebookDebugger struct {
	AppID     string
	AppSecret string
}

// DebugUserToken analyzes a user's token and permissions
func (fd *FacebookDebugger) DebugUserToken(userToken string) {
	log.Printf("🔍 Starting Facebook API diagnostics...")

	// 1. Check token validity and permissions
	fd.checkTokenInfo(userToken)

	// 2. Check user's granted permissions
	fd.checkUserPermissions(userToken)

	// 3. Check user's basic info
	fd.checkUserInfo(userToken)

	// 4. Check user's pages (raw)
	fd.checkUserPages(userToken)

	// 5. Check app info
	fd.checkAppInfo()
}

func (fd *FacebookDebugger) checkTokenInfo(token string) {
	log.Printf("📋 Checking token information...")

	url := fmt.Sprintf("https://graph.facebook.com/v23.0/debug_token?input_token=%s&access_token=%s|%s",
		token, fd.AppID, fd.AppSecret)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("❌ Error checking token info: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading token info response: %v", err)
		return
	}

	log.Printf("🔍 Token info response: %s", string(body))

	var tokenInfo struct {
		Data struct {
			AppID     string   `json:"app_id"`
			Type      string   `json:"type"`
			IsValid   bool     `json:"is_valid"`
			Scopes    []string `json:"scopes"`
			ExpiresAt int64    `json:"expires_at"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		log.Printf("⚠️ Could not parse token info: %v", err)
		return
	}

	log.Printf("✅ Token Analysis:")
	log.Printf("   Valid: %v", tokenInfo.Data.IsValid)
	log.Printf("   Type: %s", tokenInfo.Data.Type)
	log.Printf("   App ID: %s", tokenInfo.Data.AppID)
	log.Printf("   Scopes: %v", tokenInfo.Data.Scopes)
	if tokenInfo.Data.ExpiresAt > 0 {
		log.Printf("   Expires: %d", tokenInfo.Data.ExpiresAt)
	} else {
		log.Printf("   Expires: Never (permanent)")
	}
}

func (fd *FacebookDebugger) checkUserPermissions(token string) {
	log.Printf("📋 Checking user permissions...")

	url := fmt.Sprintf("https://graph.facebook.com/v23.0/me/permissions?access_token=%s", token)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("❌ Error checking permissions: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading permissions response: %v", err)
		return
	}

	log.Printf("🔍 Permissions response: %s", string(body))

	var permissions struct {
		Data []struct {
			Permission string `json:"permission"`
			Status     string `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &permissions); err != nil {
		log.Printf("⚠️ Could not parse permissions: %v", err)
		return
	}

	log.Printf("✅ Granted Permissions:")
	for _, perm := range permissions.Data {
		status := "❌"
		if perm.Status == "granted" {
			status = "✅"
		}
		log.Printf("   %s %s (%s)", status, perm.Permission, perm.Status)
	}

	// Check required permissions
	requiredPermissions := []string{
		"pages_show_list",
		"pages_manage_metadata",
		"pages_messaging",
		"instagram_basic",
		"instagram_manage_messages",
	}

	grantedMap := make(map[string]bool)
	for _, perm := range permissions.Data {
		if perm.Status == "granted" {
			grantedMap[perm.Permission] = true
		}
	}

	log.Printf("🔍 Required Permission Status:")
	for _, req := range requiredPermissions {
		status := "❌ MISSING"
		if grantedMap[req] {
			status = "✅ GRANTED"
		}
		log.Printf("   %s: %s", req, status)
	}
}

func (fd *FacebookDebugger) checkUserInfo(token string) {
	log.Printf("📋 Checking user info...")

	url := fmt.Sprintf("https://graph.facebook.com/v23.0/me?fields=id,name,email&access_token=%s", token)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("❌ Error checking user info: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading user info response: %v", err)
		return
	}

	log.Printf("👤 User info response: %s", string(body))
}

func (fd *FacebookDebugger) checkUserPages(token string) {
	log.Printf("📋 Checking user pages...")

	url := fmt.Sprintf("https://graph.facebook.com/v23.0/me/accounts?access_token=%s", token)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("❌ Error checking pages: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading pages response: %v", err)
		return
	}

	log.Printf("📄 Pages response: %s", string(body))

	var pages struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &pages); err != nil {
		log.Printf("⚠️ Could not parse pages: %v", err)
		return
	}

	if len(pages.Data) == 0 {
		log.Printf("❌ No pages found for this user")
		log.Printf("💡 This could mean:")
		log.Printf("   - User is not an admin of any Facebook pages")
		log.Printf("   - User hasn't created any Facebook pages")
		log.Printf("   - Pages are restricted or suspended")
		log.Printf("   - Required permissions not granted")
	} else {
		log.Printf("✅ Found %d pages:", len(pages.Data))
		for _, page := range pages.Data {
			log.Printf("   - %s (ID: %s)", page.Name, page.ID)
		}
	}
}

func (fd *FacebookDebugger) checkAppInfo() {
	log.Printf("📋 Checking app info...")

	url := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?access_token=%s|%s",
		fd.AppID, fd.AppID, fd.AppSecret)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("❌ Error checking app info: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading app info response: %v", err)
		return
	}

	log.Printf("📱 App info response: %s", string(body))
}

// Example usage function - creates a new debugger instance
func newFacebookDebugger() *FacebookDebugger {
	return &FacebookDebugger{
		AppID:     os.Getenv("FACEBOOK_APP_ID"),
		AppSecret: os.Getenv("FACEBOOK_APP_SECRET"),
	}
}
