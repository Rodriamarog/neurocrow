package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// ContentManagement handles Facebook and Instagram content management operations
type ContentManagement struct {
	db *sql.DB
}

// Page represents a connected Facebook or Instagram page
type Page struct {
	ID          string `json:"id"`
	PageID      string `json:"page_id"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	AccessToken string `json:"access_token"`
	ClientID    string `json:"client_id"`
}

// Post represents a Facebook or Instagram post
type Post struct {
	ID           string    `json:"id"`
	Message      string    `json:"message"`
	CreatedTime  time.Time `json:"created_time"`
	Likes        int       `json:"likes"`
	Comments     int       `json:"comments"`
	Shares       int       `json:"shares"`
	Picture      string    `json:"picture,omitempty"`
	FullPicture  string    `json:"full_picture,omitempty"`
	Platform     string    `json:"platform"`
	PageID       string    `json:"page_id"`
}

// Comment represents a comment on a post
type Comment struct {
	ID          string    `json:"id"`
	Message     string    `json:"message"`
	CreatedTime time.Time `json:"created_time"`
	From        struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"from"`
	CanReply bool `json:"can_reply"`
}

// PostRequest represents a request to create a new post
type PostRequest struct {
	Message string `json:"message"`
	ImageURL string `json:"image_url,omitempty"`
}

// CommentReply represents a reply to a comment
type CommentReply struct {
	Message string `json:"message"`
}

// NewContentManagement creates a new content management instance
func NewContentManagement(db *sql.DB) *ContentManagement {
	return &ContentManagement{db: db}
}

// GetUserPages retrieves all connected pages for a user
func (cm *ContentManagement) GetUserPages(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusUnauthorized)
		return
	}

	LogInfo("🔍 Getting pages for client: %s", clientID)

	query := `
		SELECT id, page_id, page_name, platform, access_token, client_id 
		FROM social_pages 
		WHERE client_id = $1 AND status = 'active'
		ORDER BY platform, page_name
	`

	rows, err := cm.db.Query(query, clientID)
	if err != nil {
		LogError("Error querying pages: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var page Page
		err := rows.Scan(&page.ID, &page.PageID, &page.Name, &page.Platform, &page.AccessToken, &page.ClientID)
		if err != nil {
			LogError("Error scanning page: %v", err)
			continue
		}
		// Don't expose access token in API response
		page.AccessToken = ""
		pages = append(pages, page)
	}

	LogInfo("✅ Found %d pages for client %s", len(pages), clientID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pages": pages,
	})
}

// GetPagePosts retrieves recent posts for a specific page
func (cm *ContentManagement) GetPagePosts(w http.ResponseWriter, r *http.Request) {
	// Extract pageID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Page ID required", http.StatusBadRequest)
		return
	}
	pageID := parts[0]
	
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusUnauthorized)
		return
	}

	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "10"
	}

	LogInfo("📱 Getting posts for page %s, limit %s", pageID, limit)

	// Get page access token
	accessToken, platform, err := cm.getPageAccessToken(pageID, clientID)
	if err != nil {
		LogError("Error getting page access token: %v", err)
		http.Error(w, "Page not found or access denied", http.StatusForbidden)
		return
	}

	posts, err := cm.fetchPostsFromAPI(pageID, platform, accessToken, limit)
	if err != nil {
		LogError("Error fetching posts from API: %v", err)
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}

	LogInfo("✅ Retrieved %d posts for page %s", len(posts), pageID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"posts": posts,
		"page_id": pageID,
		"platform": platform,
	})
}

// CreatePost creates a new post on Facebook or Instagram
func (cm *ContentManagement) CreatePost(w http.ResponseWriter, r *http.Request) {
	// Extract pageID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Page ID required", http.StatusBadRequest)
		return
	}
	pageID := parts[0]
	
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusUnauthorized)
		return
	}

	var postReq PostRequest
	if err := json.NewDecoder(r.Body).Decode(&postReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if postReq.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	LogInfo("📝 Creating post for page %s: %s", pageID, postReq.Message[:min(50, len(postReq.Message))])

	// Get page access token
	accessToken, platform, err := cm.getPageAccessToken(pageID, clientID)
	if err != nil {
		LogError("Error getting page access token: %v", err)
		http.Error(w, "Page not found or access denied", http.StatusForbidden)
		return
	}

	postID, err := cm.createPostOnAPI(pageID, platform, accessToken, postReq)
	if err != nil {
		LogError("Error creating post on API: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create post: %v", err), http.StatusInternalServerError)
		return
	}

	LogInfo("✅ Created post %s on page %s", postID, pageID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"post_id": postID,
		"message": "Post created successfully",
	})
}

// GetPostComments retrieves comments for a specific post
func (cm *ContentManagement) GetPostComments(w http.ResponseWriter, r *http.Request) {
	// Extract postID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/comments/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Post ID required", http.StatusBadRequest)
		return
	}
	postID := parts[0]
	
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusUnauthorized)
		return
	}

	LogInfo("💬 Getting comments for post %s", postID)

	// For now, we'll need to determine which page this post belongs to
	// In a real implementation, you'd store post metadata or parse the post ID
	comments, err := cm.fetchCommentsFromAPI(postID)
	if err != nil {
		LogError("Error fetching comments: %v", err)
		http.Error(w, "Failed to fetch comments", http.StatusInternalServerError)
		return
	}

	LogInfo("✅ Retrieved %d comments for post %s", len(comments), postID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"comments": comments,
		"post_id": postID,
	})
}

// ReplyToComment replies to a specific comment
func (cm *ContentManagement) ReplyToComment(w http.ResponseWriter, r *http.Request) {
	// Extract commentID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/comments/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "reply" {
		http.Error(w, "Comment ID required for reply", http.StatusBadRequest)
		return
	}
	commentID := parts[0]
	
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusUnauthorized)
		return
	}

	var reply CommentReply
	if err := json.NewDecoder(r.Body).Decode(&reply); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if reply.Message == "" {
		http.Error(w, "Reply message is required", http.StatusBadRequest)
		return
	}

	LogInfo("↩️ Replying to comment %s: %s", commentID, reply.Message[:min(50, len(reply.Message))])

	replyID, err := cm.replyToCommentOnAPI(commentID, reply.Message)
	if err != nil {
		LogError("Error replying to comment: %v", err)
		http.Error(w, fmt.Sprintf("Failed to reply: %v", err), http.StatusInternalServerError)
		return
	}

	LogInfo("✅ Created reply %s to comment %s", replyID, commentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"reply_id": replyID,
		"message": "Reply posted successfully",
	})
}

// Helper function to get page access token
func (cm *ContentManagement) getPageAccessToken(pageID, clientID string) (string, string, error) {
	query := `
		SELECT access_token, platform 
		FROM social_pages 
		WHERE page_id = $1 AND client_id = $2 AND status = 'active'
	`
	
	var accessToken, platform string
	err := cm.db.QueryRow(query, pageID, clientID).Scan(&accessToken, &platform)
	if err != nil {
		return "", "", fmt.Errorf("page not found: %v", err)
	}
	
	return accessToken, platform, nil
}

// Helper function to fetch posts from Facebook/Instagram API
func (cm *ContentManagement) fetchPostsFromAPI(pageID, platform, accessToken, limit string) ([]Post, error) {
	var apiURL string
	var fields string
	
	if platform == "facebook" {
		fields = "id,message,created_time,likes.summary(total_count),comments.summary(total_count),shares,picture,full_picture"
		apiURL = fmt.Sprintf("https://graph.facebook.com/v18.0/%s/posts?fields=%s&limit=%s&access_token=%s", 
			pageID, fields, limit, accessToken)
	} else if platform == "instagram" {
		fields = "id,caption,media_type,media_url,thumbnail_url,timestamp,like_count,comments_count"
		apiURL = fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media?fields=%s&limit=%s&access_token=%s", 
			pageID, fields, limit, accessToken)
	} else {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	LogInfo("🔗 Fetching posts from API for page %s (%s)", pageID, platform)
	LogDebug("🔗 API URL: %s", strings.ReplaceAll(apiURL, accessToken, "***TOKEN***"))

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != 200 {
		LogError("❌ Facebook API error %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	LogDebug("✅ Facebook API response: %s", string(body))

	var apiResponse struct {
		Data []json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		LogError("❌ Failed to parse API response: %v", err)
		return nil, fmt.Errorf("failed to parse API response: %v", err)
	}

	LogInfo("📊 Found %d posts in API response for page %s", len(apiResponse.Data), pageID)

	var posts []Post
	for _, rawPost := range apiResponse.Data {
		post, err := cm.parsePost(rawPost, platform, pageID)
		if err != nil {
			LogError("Error parsing post: %v", err)
			continue
		}
		posts = append(posts, post)
	}

	LogInfo("✅ Successfully parsed %d posts for page %s", len(posts), pageID)
	return posts, nil
}

// Helper function to parse post data based on platform
func (cm *ContentManagement) parsePost(rawPost json.RawMessage, platform, pageID string) (Post, error) {
	var post Post
	post.Platform = platform
	post.PageID = pageID

	if platform == "facebook" {
		var fbPost struct {
			ID          string    `json:"id"`
			Message     string    `json:"message"`
			CreatedTime time.Time `json:"created_time"`
			Picture     string    `json:"picture"`
			FullPicture string    `json:"full_picture"`
			Likes       struct {
				Summary struct {
					TotalCount int `json:"total_count"`
				} `json:"summary"`
			} `json:"likes"`
			Comments struct {
				Summary struct {
					TotalCount int `json:"total_count"`
				} `json:"summary"`
			} `json:"comments"`
			Shares struct {
				Count int `json:"count"`
			} `json:"shares"`
		}

		if err := json.Unmarshal(rawPost, &fbPost); err != nil {
			return post, err
		}

		post.ID = fbPost.ID
		post.Message = fbPost.Message
		post.CreatedTime = fbPost.CreatedTime
		post.Picture = fbPost.Picture
		post.FullPicture = fbPost.FullPicture
		post.Likes = fbPost.Likes.Summary.TotalCount
		post.Comments = fbPost.Comments.Summary.TotalCount
		post.Shares = fbPost.Shares.Count

	} else if platform == "instagram" {
		var igPost struct {
			ID            string    `json:"id"`
			Caption       string    `json:"caption"`
			MediaURL      string    `json:"media_url"`
			ThumbnailURL  string    `json:"thumbnail_url"`
			Timestamp     time.Time `json:"timestamp"`
			LikeCount     int       `json:"like_count"`
			CommentsCount int       `json:"comments_count"`
		}

		if err := json.Unmarshal(rawPost, &igPost); err != nil {
			return post, err
		}

		post.ID = igPost.ID
		post.Message = igPost.Caption
		post.CreatedTime = igPost.Timestamp
		post.Picture = igPost.ThumbnailURL
		post.FullPicture = igPost.MediaURL
		post.Likes = igPost.LikeCount
		post.Comments = igPost.CommentsCount
	}

	return post, nil
}

// Helper function to create post on Facebook/Instagram API
func (cm *ContentManagement) createPostOnAPI(pageID, platform, accessToken string, postReq PostRequest) (string, error) {
	if platform == "facebook" {
		return cm.createFacebookPost(pageID, accessToken, postReq)
	} else if platform == "instagram" {
		// Instagram posting requires a two-step process: create media object, then publish
		return cm.createInstagramPost(pageID, accessToken, postReq)
	} else {
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
}

// Helper function to create Facebook post
func (cm *ContentManagement) createFacebookPost(pageID, accessToken string, postReq PostRequest) (string, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/feed", pageID)
	
	// Prepare form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	writer.WriteField("message", postReq.Message)
	writer.WriteField("access_token", accessToken)
	
	if postReq.ImageURL != "" {
		writer.WriteField("link", postReq.ImageURL)
	}
	
	writer.Close()

	LogDebug("🔗 Creating Facebook post: %s", apiURL)
	LogDebug("📤 Post message: %s", postReq.Message[:min(100, len(postReq.Message))])

	req, err := http.NewRequest("POST", apiURL, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	LogDebug("📥 Facebook post response: %d - %s", resp.StatusCode, string(body))

	if resp.StatusCode != 200 {
		// Try to parse error response
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error.Message != "" {
			return "", fmt.Errorf("Facebook API error: %s (Type: %s, Code: %d)", 
				errorResp.Error.Message, errorResp.Error.Type, errorResp.Error.Code)
		}
		
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if response.ID == "" {
		return "", fmt.Errorf("no post ID returned in response")
	}

	return response.ID, nil
}

// Helper function to create Instagram post (simplified version)
func (cm *ContentManagement) createInstagramPost(pageID, accessToken string, postReq PostRequest) (string, error) {
	// Instagram posting is more complex and requires media upload first
	// For now, return an error indicating it's not fully implemented
	return "", fmt.Errorf("Instagram posting not fully implemented yet - requires media container creation")
}

// Helper function to fetch comments from Facebook/Instagram API
func (cm *ContentManagement) fetchCommentsFromAPI(postID string) ([]Comment, error) {
	// Determine platform based on post ID format
	// Facebook post IDs are typically in format: pageId_postId
	// Instagram post IDs are typically numeric
	
	var platform string
	
	// For now, assume Facebook if contains underscore, Instagram otherwise
	if strings.Contains(postID, "_") {
		platform = "facebook"
	} else {
		platform = "instagram"
	}
	
	LogDebug("🔍 Fetching comments for %s post: %s", platform, postID)
	
	// TODO: Get access token for the post's page
	// For now, return empty comments with a message
	LogInfo("⚠️ Comment fetching requires page access token lookup - not fully implemented")
	
	return []Comment{}, nil
}

// Helper function to reply to comment (simplified)
func (cm *ContentManagement) replyToCommentOnAPI(commentID, message string) (string, error) {
	// This is a placeholder - in a real implementation, you'd:
	// 1. Determine platform and get access token
	// 2. Make API call to reply to comment
	// 3. Return reply ID
	
	return "", fmt.Errorf("comment reply not fully implemented yet")
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}