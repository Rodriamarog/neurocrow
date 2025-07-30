import React, { useState, useEffect } from 'react';
import './Insights.css';

function Insights() {
  const [connectedPages, setConnectedPages] = useState([]);
  const [selectedPage, setSelectedPage] = useState(null);
  const [selectedLimit, setSelectedLimit] = useState('10');
  const [postsData, setPostsData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Check for connected pages on component mount
  useEffect(() => {
    checkConnectedPages();
  }, []);

  // Fetch posts when page or limit changes
  useEffect(() => {
    if (selectedPage) {
      fetchPosts(selectedPage.page_id, selectedLimit);
    }
  }, [selectedPage, selectedLimit]);

  const checkConnectedPages = async () => {
    try {
      console.log('🔍 Fetching connected pages from API...');
      const response = await fetch(
        'https://neurocrow-client-manager.onrender.com/pages'
      );
      
      console.log('📡 Pages API response status:', response.status);
      const responseText = await response.text();
      console.log('📡 Pages API raw response:', responseText);
      
      if (response.ok) {
        let data;
        try {
          data = JSON.parse(responseText);
          console.log('✅ Pages API parsed data:', data);
          setConnectedPages(data.pages || []);
          if (data.pages && data.pages.length > 0) {
            setSelectedPage(data.pages[0]);
            console.log('📄 Selected first page:', data.pages[0]);
          } else {
            console.log('⚠️ No pages found in API response');
          }
        } catch (parseError) {
          console.error('❌ Failed to parse pages API response:', parseError);
          setConnectedPages([]);
        }
      } else {
        console.error('❌ Pages API call failed:', response.status, responseText);
        setConnectedPages([]);
      }
      setLoading(false);
    } catch (error) {
      console.error('❌ Error checking connected pages:', error);
      setConnectedPages([]);
      setLoading(false);
    }
  };

  const fetchPosts = async (pageId, limit) => {
    setLoading(true);
    setError(null);
    
    try {
      console.log(`🔍 Fetching posts for page ${pageId}, limit ${limit}...`);
      const response = await fetch(
        `https://neurocrow-client-manager.onrender.com/posts?pageId=${pageId}&limit=${limit}`
      );
      
      console.log('📱 Posts API response status:', response.status);
      const responseText = await response.text();
      console.log('📱 Posts API raw response:', responseText);
      
      if (!response.ok) {
        throw new Error(`API Error (${response.status}): ${responseText}`);
      }
      
      // Try to parse as JSON
      let data;
      try {
        data = JSON.parse(responseText);
        console.log('✅ Posts API parsed data:', data);
        setPostsData(data);
      } catch (parseError) {
        console.error('❌ JSON Parse Error:', parseError);
        throw new Error(`Invalid JSON response: ${responseText.substring(0, 200)}...`);
      }
    } catch (error) {
      console.error('❌ Error fetching posts:', error);
      setError(error.message);
    } finally {
      setLoading(false);
    }
  };

  const handleConnectFacebook = () => {
    window.location.href = '/login';
  };


  if (loading && connectedPages.length === 0) {
    return (
      <div className="insights-container">
        <div className="loading-state">
          <i className="fas fa-spinner fa-spin"></i>
          <p>Loading your latest posts...</p>
        </div>
      </div>
    );
  }

  if (connectedPages.length === 0) {
    return (
      <div className="insights-container">
        <div className="no-pages-state">
          <i className="fab fa-facebook fa-3x"></i>
          <h2>Connect Your Facebook Pages</h2>
          <p>To view your latest posts and engagement data, you need to connect your Facebook pages first.</p>
          <p>Our posts dashboard will show you:</p>
          <ul>
            <li>📱 Recent posts from your pages</li>
            <li>❤️ Likes, comments, and shares on each post</li>
            <li>📅 Post timestamps and activity timeline</li>
            <li>📝 Post content and page updates</li>
            <li>📊 Engagement metrics per post</li>
          </ul>
          <button onClick={handleConnectFacebook} className="connect-btn">
            <i className="fab fa-facebook"></i> Connect Facebook Pages
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="insights-container">
      <div className="insights-header">
        <h1>📱 Latest Posts</h1>
        <div className="insights-controls">
          <select 
            value={selectedPage?.page_id || ''} 
            onChange={(e) => {
              const page = connectedPages.find(p => p.page_id === e.target.value);
              setSelectedPage(page);
            }}
            className="page-selector"
          >
            {connectedPages.map(page => (
              <option key={page.page_id} value={page.page_id}>
                {page.name} ({page.platform})
              </option>
            ))}
          </select>
          
          <select 
            value={selectedLimit} 
            onChange={(e) => setSelectedLimit(e.target.value)}
            className="period-selector"
          >
            <option value="5">Last 5 Posts</option>
            <option value="10">Last 10 Posts</option>
            <option value="20">Last 20 Posts</option>
          </select>
        </div>
      </div>

      {error && (
        <div className="error-state">
          <i className="fas fa-exclamation-triangle"></i>
          <p>Error loading posts: {error}</p>
          <button onClick={() => fetchPosts(selectedPage.page_id, selectedLimit)}>
            Try Again
          </button>
        </div>
      )}

      {loading ? (
        <div className="loading-state">
          <i className="fas fa-spinner fa-spin"></i>
          <p>Loading posts for {selectedPage?.name}...</p>
        </div>
      ) : postsData ? (
        <div className="insights-content">
          {/* Page Summary */}
          <div className="page-summary">
            <h2>📄 {postsData.page_name}</h2>
            <p>Platform: {postsData.platform} • Total Posts Found: {postsData.count}</p>
          </div>

          {/* Posts Feed */}
          <div className="posts-feed">
            {postsData.posts && postsData.posts.length > 0 ? (
              postsData.posts.map((post, index) => (
                <div key={post.id} className="post-card">
                  <div className="post-header">
                    <div className="post-author">
                      <i className="fab fa-facebook"></i>
                      <span>{post.from.name}</span>
                    </div>
                    <div className="post-date">
                      {new Date(post.created_time).toLocaleDateString('en-US', {
                        year: 'numeric',
                        month: 'short',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit'
                      })}
                    </div>
                  </div>
                  
                  <div className="post-content">
                    {post.message && (
                      <p className="post-message">{post.message}</p>
                    )}
                    {post.story && (
                      <p className="post-story">
                        <i className="fas fa-info-circle"></i>
                        {post.story}
                      </p>
                    )}
                  </div>
                  
                  <div className="post-engagement">
                    <div className="engagement-item">
                      <i className="fas fa-thumbs-up"></i>
                      <span>{post.likes?.summary?.total_count || 0} Likes</span>
                    </div>
                    <div className="engagement-item">
                      <i className="fas fa-comment"></i>
                      <span>{post.comments?.summary?.total_count || 0} Comments</span>
                    </div>
                    {post.shares && (
                      <div className="engagement-item">
                        <i className="fas fa-share"></i>
                        <span>{post.shares.count} Shares</span>
                      </div>
                    )}
                  </div>
                </div>
              ))
            ) : (
              <div className="no-posts">
                <i className="fas fa-file-alt"></i>
                <h3>No posts found</h3>
                <p>This page doesn't have any recent posts to display.</p>
              </div>
            )}
          </div>

          <div className="feature-note">
            <h4>💡 About This Feature</h4>
            <p>This dashboard demonstrates the legitimate business use of Facebook's <code>pages_read_engagement</code> permission:</p>
            <ul>
              <li><strong>Get content posted by your Page</strong> - View recent posts and updates</li>
              <li><strong>Read engagement metrics</strong> - See likes, comments, and shares</li>
              <li><strong>Manage Page content</strong> - Monitor post performance and engagement</li>
              <li><strong>Page administration</strong> - Help Page Admins manage their content effectively</li>
            </ul>
            <p>All data is retrieved directly from Facebook's Pages API using legitimate page management permissions.</p>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default Insights;