import React, { useState, useEffect } from 'react';

function ContentManager() {
  const [connectedPages, setConnectedPages] = useState([]);
  const [selectedPage, setSelectedPage] = useState(null);
  const [posts, setPosts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [postComposerOpen, setPostComposerOpen] = useState(false);
  const [newPostMessage, setNewPostMessage] = useState('');
  const [selectedFiles, setSelectedFiles] = useState([]);
  const [filePreviewUrls, setFilePreviewUrls] = useState([]);
  const [publishingPost, setPublishingPost] = useState(false);
  const [commentsVisible, setCommentsVisible] = useState({});
  const [comments, setComments] = useState({});
  const [loadingComments, setLoadingComments] = useState({});
  const [newComment, setNewComment] = useState({});
  const [editingComment, setEditingComment] = useState(null);
  const [editCommentText, setEditCommentText] = useState('');
  const [replyingToComment, setReplyingToComment] = useState(null);
  const [replyText, setReplyText] = useState('');
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [postToDelete, setPostToDelete] = useState(null);
  const [deletingPost, setDeletingPost] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMorePosts, setHasMorePosts] = useState(true);

  // Get client ID for authentication
  const getClientId = () => {
    return localStorage.getItem('client_id') || '';
  };

  // Helper function to get authentication headers
  const getAuthHeaders = () => {
    const clientId = getClientId();
    if (!clientId) {
      return {};
    }
    
    return {
      'X-Client-ID': clientId,
      'Content-Type': 'application/json'
    };
  };

  // Check for connected pages on component mount
  useEffect(() => {
    const clientId = getClientId();
    if (clientId) {
      fetchConnectedPages();
    } else {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Fetch posts when page changes
  useEffect(() => {
    if (selectedPage) {
      fetchPosts(selectedPage.page_id);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedPage]);

  const fetchConnectedPages = async () => {
    try {
      console.log('🔍 Fetching connected pages for content management...');
      const authHeaders = getAuthHeaders();
      
      if (!authHeaders['X-Client-ID']) {
        console.log('❌ No client ID found, user not authenticated');
        setConnectedPages([]);
        setLoading(false);
        return;
      }
      
      const response = await fetch(
        'https://neurocrow-message-router.onrender.com/api/pages',
        {
          method: 'GET',
          headers: authHeaders
        }
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
            // Filter for Urban Edge pages for demo
            const urbanEdgePages = data.pages.filter(page => 
              page.name.toLowerCase().includes('urban edge') || 
              page.name.toLowerCase().includes('urban.edge')
            );
            if (urbanEdgePages.length > 0) {
              setSelectedPage(urbanEdgePages[0]);
              console.log('📄 Selected Urban Edge page:', urbanEdgePages[0]);
            } else {
              setSelectedPage(data.pages[0]);
              console.log('📄 Selected first page:', data.pages[0]);
            }
          } else {
            console.log('⚠️ No pages found in API response');
          }
        } catch (parseError) {
          console.error('❌ Failed to parse pages API response:', parseError);
          setConnectedPages([]);
        }
      } else {
        console.error('❌ Pages API call failed:', response.status, responseText);
        setError(`Failed to fetch pages: ${response.status} ${responseText}`);
        setConnectedPages([]);
      }
      setLoading(false);
    } catch (error) {
      console.error('❌ Error checking connected pages:', error);
      setError(`Error connecting to server: ${error.message}`);
      setConnectedPages([]);
      setLoading(false);
    }
  };

  const fetchPosts = async (pageId, append = false, limit = 20) => {
    if (append) {
      setLoadingMore(true);
    } else {
      setLoading(true);
      setHasMorePosts(true);
    }
    setError(null);

    try {
      const offset = append ? posts.length : 0;
      console.log(`🔍 Fetching posts for page ${pageId} (offset: ${offset}, limit: ${limit})...`);
      const authHeaders = getAuthHeaders();

      if (!authHeaders['X-Client-ID']) {
        throw new Error('No authentication available');
      }

      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/posts/${pageId}?limit=${limit}&offset=${offset}`,
        {
          method: 'GET',
          headers: authHeaders
        }
      );

      console.log('📱 Posts API response status:', response.status);
      const responseText = await response.text();
      console.log('📱 Posts API raw response:', responseText);

      if (!response.ok) {
        throw new Error(`Posts API Error (${response.status}): ${responseText}`);
      }

      // Try to parse as JSON
      let data;
      try {
        data = JSON.parse(responseText);
        console.log('✅ Posts API parsed data:', data);
        const newPosts = data.posts || [];

        if (append) {
          setPosts(prevPosts => [...prevPosts, ...newPosts]);
        } else {
          setPosts(newPosts);
        }

        // Check if there are more posts to load
        setHasMorePosts(newPosts.length === limit);
      } catch (parseError) {
        console.error('❌ JSON Parse Error:', parseError);
        throw new Error(`Invalid JSON response: ${responseText.substring(0, 200)}...`);
      }
    } catch (error) {
      console.error('❌ Error fetching posts:', error);
      setError(error.message);
      if (!append) {
        setPosts([]);
      }
    } finally {
      if (append) {
        setLoadingMore(false);
      } else {
        setLoading(false);
      }
    }
  };

  const handleFileSelect = (event) => {
    const files = Array.from(event.target.files);
    const validFiles = files.filter(file => {
      const isImage = file.type.startsWith('image/');
      const isValidSize = file.size <= 10 * 1024 * 1024; // 10MB limit
      if (!isImage) {
        setError('Only image files are allowed');
        return false;
      }
      if (!isValidSize) {
        setError('File size must be under 10MB');
        return false;
      }
      return true;
    });

    if (validFiles.length > 0) {
      setSelectedFiles(validFiles);
      
      // Create preview URLs
      const previewUrls = validFiles.map(file => URL.createObjectURL(file));
      setFilePreviewUrls(previewUrls);
      setError(null);
    }
  };

  const removeFile = (index) => {
    const newFiles = selectedFiles.filter((_, i) => i !== index);
    const newPreviews = filePreviewUrls.filter((_, i) => i !== index);
    
    // Revoke the URL to prevent memory leaks
    URL.revokeObjectURL(filePreviewUrls[index]);
    
    setSelectedFiles(newFiles);
    setFilePreviewUrls(newPreviews);
  };

  const handleCreatePost = async () => {
    if ((!newPostMessage.trim() && selectedFiles.length === 0) || !selectedPage) {
      return;
    }

    setPublishingPost(true);
    setError(null);

    try {
      console.log(`📝 Creating post for page ${selectedPage.page_id}: ${newPostMessage}`);
      const authHeaders = getAuthHeaders();

      // Create FormData for multipart upload
      const formData = new FormData();
      formData.append('message', newPostMessage);
      
      // Add files if any are selected
      selectedFiles.forEach((file, index) => {
        formData.append(`media_${index}`, file);
      });

      // Remove Content-Type header to let browser set it automatically for FormData
      const { 'Content-Type': contentType, ...headersWithoutContentType } = authHeaders;

      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/posts/${selectedPage.page_id}`,
        {
          method: 'POST',
          headers: headersWithoutContentType,
          body: formData
        }
      );

      const responseText = await response.text();
      console.log('📤 Post creation response:', response.status, responseText);

      if (response.ok) {
        console.log('✅ Post created successfully');
        setNewPostMessage('');
        
        // Clean up file previews
        filePreviewUrls.forEach(url => URL.revokeObjectURL(url));
        setSelectedFiles([]);
        setFilePreviewUrls([]);
        
        setPostComposerOpen(false);
        // Refresh posts
        fetchPosts(selectedPage.page_id);
      } else {
        throw new Error(`Failed to create post: ${response.status} ${responseText}`);
      }
    } catch (error) {
      console.error('❌ Error creating post:', error);
      setError(`Failed to create post: ${error.message}`);
    } finally {
      setPublishingPost(false);
    }
  };

  const handleConnectFacebook = () => {
    window.location.href = '/login';
  };

  // Comment management functions
  const fetchComments = async (postId) => {
    if (!selectedPage) return;

    setLoadingComments(prev => ({ ...prev, [postId]: true }));
    try {
      const authHeaders = getAuthHeaders();
      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/comments/${selectedPage.page_id}/${postId}`,
        {
          method: 'GET',
          headers: authHeaders
        }
      );

      if (response.ok) {
        const data = await response.json();
        setComments(prev => ({ ...prev, [postId]: data.comments || [] }));
      } else {
        console.error('Failed to fetch comments:', response.status);
      }
    } catch (error) {
      console.error('Error fetching comments:', error);
    } finally {
      setLoadingComments(prev => ({ ...prev, [postId]: false }));
    }
  };

  const toggleComments = async (postId) => {
    const isVisible = commentsVisible[postId];
    setCommentsVisible(prev => ({ ...prev, [postId]: !isVisible }));
    
    if (!isVisible && !comments[postId]) {
      await fetchComments(postId);
    }
  };

  const addComment = async (postId) => {
    const commentText = newComment[postId];
    if (!commentText?.trim()) return;

    try {
      const authHeaders = getAuthHeaders();
      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/comments/${selectedPage.page_id}/${postId}`,
        {
          method: 'POST',
          headers: authHeaders,
          body: JSON.stringify({ message: commentText })
        }
      );

      if (response.ok) {
        setNewComment(prev => ({ ...prev, [postId]: '' }));
        await fetchComments(postId);
      }
    } catch (error) {
      console.error('Error adding comment:', error);
    }
  };

  const editComment = async (commentId) => {
    if (!editCommentText.trim()) return;

    try {
      const authHeaders = getAuthHeaders();
      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/comments/${commentId}`,
        {
          method: 'PUT',
          headers: authHeaders,
          body: JSON.stringify({ message: editCommentText })
        }
      );

      if (response.ok) {
        setEditingComment(null);
        setEditCommentText('');
        // Refresh comments for the affected post
        Object.keys(comments).forEach(postId => {
          if (comments[postId].some(c => c.id === commentId)) {
            fetchComments(postId);
          }
        });
      }
    } catch (error) {
      console.error('Error editing comment:', error);
    }
  };

  const deleteComment = async (commentId) => {
    if (!window.confirm('Are you sure you want to delete this comment?')) return;

    try {
      const authHeaders = getAuthHeaders();
      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/comments/${commentId}`,
        {
          method: 'DELETE',
          headers: authHeaders
        }
      );

      if (response.ok) {
        // Refresh comments for the affected post
        Object.keys(comments).forEach(postId => {
          if (comments[postId].some(c => c.id === commentId)) {
            fetchComments(postId);
          }
        });
      }
    } catch (error) {
      console.error('Error deleting comment:', error);
    }
  };

  const replyToComment = async (commentId) => {
    if (!replyText.trim()) return;

    try {
      const authHeaders = getAuthHeaders();
      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/comments/${commentId}/reply`,
        {
          method: 'POST',
          headers: authHeaders,
          body: JSON.stringify({ message: replyText })
        }
      );

      if (response.ok) {
        setReplyingToComment(null);
        setReplyText('');
        // Refresh comments for the affected post
        Object.keys(comments).forEach(postId => {
          if (comments[postId].some(c => c.id === commentId)) {
            fetchComments(postId);
          }
        });
      }
    } catch (error) {
      console.error('Error replying to comment:', error);
    }
  };

  const handleDeletePost = (post) => {
    setPostToDelete(post);
    setDeleteConfirmOpen(true);
  };

  const confirmDeletePost = async () => {
    if (!postToDelete) return;

    setDeletingPost(true);
    setError(null);

    try {
      console.log(`🗑️ Deleting post ${postToDelete.id}`);
      const authHeaders = getAuthHeaders();

      if (!authHeaders['X-Client-ID']) {
        throw new Error('No authentication available');
      }

      const response = await fetch(
        `https://neurocrow-message-router.onrender.com/api/posts/${postToDelete.id}`,
        {
          method: 'DELETE',
          headers: authHeaders
        }
      );

      const responseText = await response.text();
      console.log('🗑️ Delete response:', response.status, responseText);

      if (response.ok) {
        console.log('✅ Post deleted successfully');
        // Remove the post from the list
        setPosts(prevPosts => prevPosts.filter(p => p.id !== postToDelete.id));
        setDeleteConfirmOpen(false);
        setPostToDelete(null);
      } else {
        throw new Error(`Failed to delete post: ${response.status} ${responseText}`);
      }
    } catch (error) {
      console.error('❌ Error deleting post:', error);
      setError(`Failed to delete post: ${error.message}`);
    } finally {
      setDeletingPost(false);
    }
  };

  const cancelDeletePost = () => {
    setDeleteConfirmOpen(false);
    setPostToDelete(null);
  };

  const loadMorePosts = () => {
    if (selectedPage && !loadingMore && hasMorePosts) {
      fetchPosts(selectedPage.page_id, true);
    }
  };

  if (loading && connectedPages.length === 0) {
    return (
      <div className="min-h-screen bg-slate-50 dark:bg-slate-900 flex items-center justify-center p-4">
        <div className="text-center">
          <div className="w-12 h-12 mx-auto mb-4 text-slate-600 dark:text-slate-400">
            <i className="fas fa-spinner fa-spin text-2xl"></i>
          </div>
          <p className="text-slate-600 dark:text-slate-300">Loading your pages...</p>
        </div>
      </div>
    );
  }

  if (connectedPages.length === 0) {
    return (
      <div className="min-h-screen bg-slate-50 dark:bg-slate-900 flex items-center justify-center p-4">
        <div className="w-full max-w-2xl bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg p-8 text-center">
          <div className="w-20 h-20 mx-auto mb-6 flex items-center justify-center bg-gradient-to-r from-blue-500 to-purple-500 rounded-full text-white">
            <i className="fas fa-plus-circle text-2xl"></i>
          </div>
          <h2 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-4">Connect Your Pages</h2>
          <p className="text-slate-600 dark:text-slate-300 mb-6">To manage content and engage with your audience, you need to connect your Facebook and Instagram pages first.</p>
          <div className="bg-slate-50 dark:bg-slate-700 rounded-lg p-6 mb-6">
            <p className="text-slate-700 dark:text-slate-300 font-medium mb-3">With content management you can:</p>
            <ul className="text-left space-y-2 text-slate-600 dark:text-slate-400">
              <li className="flex items-center gap-3">
                <span className="text-blue-500">📝</span>
                Create and publish posts to Facebook and Instagram
              </li>
              <li className="flex items-center gap-3">
                <span className="text-green-500">💬</span>
                Reply to comments and engage with customers
              </li>
              <li className="flex items-center gap-3">
                <span className="text-purple-500">📊</span>
                View post performance and engagement metrics
              </li>
              <li className="flex items-center gap-3">
                <span className="text-orange-500">🎯</span>
                Demonstrate your social media management capabilities
              </li>
            </ul>
          </div>
          <button 
            onClick={handleConnectFacebook} 
            className="w-full flex items-center justify-center gap-3 px-6 py-3 bg-gradient-to-r from-blue-600 to-purple-600 hover:from-blue-700 hover:to-purple-700 text-white font-medium rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            <i className="fas fa-link text-lg"></i>
            Connect Facebook & Instagram
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-900">
      {/* Header */}
      <div className="bg-white dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700 sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between py-6 gap-4">
            <div>
              <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">Content Management</h1>
              <p className="text-slate-600 dark:text-slate-400 mt-1">Manage posts and engagement for Urban Edge accounts</p>
            </div>
            
            <div className="flex flex-col sm:flex-row gap-3 sm:items-center">
              <select 
                value={selectedPage?.page_id || ''} 
                onChange={(e) => {
                  const page = connectedPages.find(p => p.page_id === e.target.value);
                  setSelectedPage(page);
                }}
                className="px-4 py-2 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 text-slate-900 dark:text-slate-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              >
                {connectedPages.map(page => (
                  <option key={page.page_id} value={page.page_id}>
                    {page.name} ({page.platform})
                  </option>
                ))}
              </select>
              
              <button 
                onClick={() => setPostComposerOpen(true)}
                disabled={!selectedPage}
                className="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-green-500 to-teal-500 hover:from-green-600 hover:to-teal-600 disabled:from-gray-400 disabled:to-gray-500 text-white font-medium rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-green-500 focus:ring-offset-2 disabled:cursor-not-allowed"
              >
                <i className="fas fa-plus text-sm"></i>
                Create Post
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Error Banner */}
      {error && (
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-4">
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 flex items-start gap-3">
            <i className="fas fa-exclamation-triangle text-red-500 flex-shrink-0 mt-0.5"></i>
            <div className="flex-1">
              <p className="text-red-700 dark:text-red-300">{error}</p>
            </div>
            <button 
              onClick={() => setError(null)}
              className="text-red-500 hover:text-red-700 dark:hover:text-red-400 font-bold text-lg leading-none"
            >
              ×
            </button>
          </div>
        </div>
      )}

      {/* Post Composer Modal */}
      {postComposerOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50" onClick={() => !publishingPost && setPostComposerOpen(false)}>
          <div className="w-full max-w-2xl bg-white dark:bg-slate-800 rounded-lg shadow-xl" onClick={(e) => e.stopPropagation()}>
            {/* Modal Header */}
            <div className="flex items-center justify-between p-6 border-b border-slate-200 dark:border-slate-700">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Create New Post</h3>
              <button 
                onClick={() => setPostComposerOpen(false)}
                disabled={publishingPost}
                className="p-1 hover:bg-slate-100 dark:hover:bg-slate-700 rounded-full transition-colors disabled:cursor-not-allowed"
              >
                <i className="fas fa-times text-slate-500 dark:text-slate-400"></i>
              </button>
            </div>
            
            {/* Modal Body */}
            <div className="p-6 space-y-4">
              <div className="flex items-center gap-3 p-3 bg-gradient-to-r from-blue-50 to-purple-50 dark:from-blue-900/20 dark:to-purple-900/20 rounded-lg border border-blue-100 dark:border-blue-800/50">
                <div className="w-8 h-8 bg-gradient-to-r from-blue-500 to-purple-500 rounded-full flex items-center justify-center">
                  <i className={`fab fa-${selectedPage?.platform} text-white text-sm`}></i>
                </div>
                <div>
                  <p className="font-medium text-slate-900 dark:text-slate-100">{selectedPage?.name}</p>
                  <p className="text-sm text-slate-500 dark:text-slate-400 capitalize">{selectedPage?.platform}</p>
                </div>
              </div>
              
              <textarea
                value={newPostMessage}
                onChange={(e) => setNewPostMessage(e.target.value)}
                placeholder="What's on your mind?"
                className="w-full p-4 border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 placeholder-slate-500 dark:placeholder-slate-400 rounded-lg resize-none focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-slate-50 disabled:cursor-not-allowed"
                rows={6}
                disabled={publishingPost}
              />

              {/* File Upload Section */}
              <div className="space-y-4">
                <div className="flex items-center gap-3">
                  <label className="flex items-center gap-2 px-4 py-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg cursor-pointer transition-colors">
                    <i className="fas fa-image text-slate-600 dark:text-slate-400"></i>
                    <span className="text-slate-700 dark:text-slate-300 font-medium">Add Photos</span>
                    <input
                      type="file"
                      multiple
                      accept="image/*"
                      onChange={handleFileSelect}
                      disabled={publishingPost}
                      className="hidden"
                    />
                  </label>
                  {selectedFiles.length > 0 && (
                    <span className="text-sm text-slate-500 dark:text-slate-400">
                      {selectedFiles.length} file{selectedFiles.length > 1 ? 's' : ''} selected
                    </span>
                  )}
                </div>

                {/* File Previews */}
                {filePreviewUrls.length > 0 && (
                  <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                    {filePreviewUrls.map((url, index) => (
                      <div key={index} className="relative group">
                        <img
                          src={url}
                          alt={`Preview ${index + 1}`}
                          className="w-full h-24 object-cover rounded-lg border border-slate-200 dark:border-slate-600"
                        />
                        <button
                          onClick={() => removeFile(index)}
                          disabled={publishingPost}
                          className="absolute -top-2 -right-2 w-6 h-6 bg-red-500 hover:bg-red-600 text-white rounded-full flex items-center justify-center text-xs transition-colors opacity-0 group-hover:opacity-100 disabled:cursor-not-allowed"
                        >
                          <i className="fas fa-times"></i>
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              
              <div className="flex justify-between items-center text-sm">
                <span className="text-slate-500 dark:text-slate-400">
                  {newPostMessage.length} characters
                </span>
                {newPostMessage.length > 2000 && (
                  <span className="text-red-500">Consider shortening your post</span>
                )}
              </div>
            </div>
            
            {/* Modal Footer */}
            <div className="flex gap-3 p-6 border-t border-slate-200 dark:border-slate-700">
              <button 
                onClick={() => setPostComposerOpen(false)}
                disabled={publishingPost}
                className="flex-1 px-4 py-2 text-slate-700 dark:text-slate-300 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 hover:bg-slate-50 dark:hover:bg-slate-600 rounded-lg transition-colors disabled:cursor-not-allowed"
              >
                Cancel
              </button>
              <button 
                onClick={handleCreatePost}
                disabled={(!newPostMessage.trim() && selectedFiles.length === 0) || publishingPost}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-gradient-to-r from-green-500 to-teal-500 hover:from-green-600 hover:to-teal-600 disabled:from-gray-400 disabled:to-gray-500 text-white font-medium rounded-lg transition-all disabled:cursor-not-allowed"
              >
                {publishingPost ? (
                  <>
                    <i className="fas fa-spinner fa-spin"></i>
                    Publishing...
                  </>
                ) : (
                  <>
                    <i className="fas fa-paper-plane"></i>
                    Publish
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteConfirmOpen && postToDelete && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50" onClick={() => !deletingPost && cancelDeletePost()}>
          <div className="w-full max-w-md bg-white dark:bg-slate-800 rounded-lg shadow-xl" onClick={(e) => e.stopPropagation()}>
            {/* Modal Header */}
            <div className="flex items-center justify-between p-6 border-b border-slate-200 dark:border-slate-700">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Delete Post</h3>
              <button
                onClick={cancelDeletePost}
                disabled={deletingPost}
                className="p-1 hover:bg-slate-100 dark:hover:bg-slate-700 rounded-full transition-colors disabled:cursor-not-allowed"
              >
                <i className="fas fa-times text-slate-500 dark:text-slate-400"></i>
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-6 space-y-4">
              <div className="flex items-center gap-3 p-3 bg-red-50 dark:bg-red-900/20 rounded-lg border border-red-200 dark:border-red-800/50">
                <div className="w-8 h-8 bg-red-500 rounded-full flex items-center justify-center">
                  <i className="fas fa-exclamation-triangle text-white text-sm"></i>
                </div>
                <div>
                  <p className="font-medium text-red-900 dark:text-red-100">Are you sure?</p>
                  <p className="text-sm text-red-700 dark:text-red-300">This action cannot be undone.</p>
                </div>
              </div>

              <div className="bg-slate-50 dark:bg-slate-700 rounded-lg p-4">
                <p className="text-sm text-slate-600 dark:text-slate-400 mb-2">You are about to delete this post:</p>
                <div className="flex items-center gap-2 mb-2">
                  <div className={`w-4 h-4 rounded-full flex items-center justify-center ${postToDelete.platform === 'facebook' ? 'bg-blue-500' : 'bg-gradient-to-br from-purple-500 to-pink-500'} text-white`}>
                    <i className={`fab fa-${postToDelete.platform} text-xs`}></i>
                  </div>
                  <span className="text-sm font-medium text-slate-700 dark:text-slate-300 capitalize">
                    {postToDelete.platform}
                  </span>
                </div>
                {postToDelete.message && (
                  <p className="text-sm text-slate-800 dark:text-slate-200 line-clamp-3">
                    {postToDelete.message}
                  </p>
                )}
              </div>
            </div>

            {/* Modal Footer */}
            <div className="flex gap-3 p-6 border-t border-slate-200 dark:border-slate-700">
              <button
                onClick={cancelDeletePost}
                disabled={deletingPost}
                className="flex-1 px-4 py-2 text-slate-700 dark:text-slate-300 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 hover:bg-slate-50 dark:hover:bg-slate-600 rounded-lg transition-colors disabled:cursor-not-allowed"
              >
                Cancel
              </button>
              <button
                onClick={confirmDeletePost}
                disabled={deletingPost}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-red-600 hover:bg-red-700 disabled:bg-red-400 text-white font-medium rounded-lg transition-all disabled:cursor-not-allowed"
              >
                {deletingPost ? (
                  <>
                    <i className="fas fa-spinner fa-spin"></i>
                    Deleting...
                  </>
                ) : (
                  <>
                    <i className="fas fa-trash"></i>
                    Delete Post
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Posts Section */}
      <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100">Recent Posts</h2>
            <p className="text-slate-600 dark:text-slate-400 mt-1">View and manage your content</p>
          </div>
          {selectedPage && (
            <button 
              onClick={() => fetchPosts(selectedPage.page_id)}
              disabled={loading}
              className="flex items-center gap-2 px-4 py-2 text-slate-600 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 bg-white dark:bg-slate-800 border border-slate-300 dark:border-slate-600 hover:border-slate-400 dark:hover:border-slate-500 rounded-lg transition-all disabled:cursor-not-allowed"
            >
              <i className={`fas fa-sync text-sm ${loading ? 'fa-spin' : ''}`}></i>
              Refresh
            </button>
          )}
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="text-center">
              <div className="w-12 h-12 mx-auto mb-4 text-slate-600 dark:text-slate-400">
                <i className="fas fa-spinner fa-spin text-2xl"></i>
              </div>
              <p className="text-slate-600 dark:text-slate-300">Loading posts...</p>
            </div>
          </div>
        ) : posts.length > 0 ? (
          <>
            <div className="flex flex-col space-y-8">
              {posts.map((post) => (
              <div key={post.id} className="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg overflow-hidden hover:shadow-lg dark:hover:shadow-slate-900/20 transition-all">
                {/* Post Header */}
                <div className="flex items-center justify-between p-4 border-b border-slate-200 dark:border-slate-700">
                  <div className="flex items-center gap-2">
                    <div className={`w-6 h-6 rounded-full flex items-center justify-center ${post.platform === 'facebook' ? 'bg-blue-500' : 'bg-gradient-to-br from-purple-500 to-pink-500'} text-white`}>
                      <i className={`fab fa-${post.platform} text-xs`}></i>
                    </div>
                    <span className="text-sm font-medium text-slate-700 dark:text-slate-300 capitalize">
                      {post.platform}
                    </span>
                  </div>
                  <span className="text-sm text-slate-500 dark:text-slate-400">
                    {new Date(post.created_time).toLocaleDateString()}
                  </span>
                </div>
                
                {/* Post Content */}
                <div className="p-4">
                  {post.message && (
                    <p className="text-slate-800 dark:text-slate-200 text-sm mb-3 line-clamp-4 text-left">
                      {post.message}
                    </p>
                  )}
                  {post.full_picture && (
                    <div className="rounded-lg overflow-hidden mb-3">
                      <img 
                        src={post.full_picture} 
                        alt="Post content" 
                        className="w-full max-h-96 object-contain bg-slate-100 dark:bg-slate-700"
                      />
                    </div>
                  )}
                </div>
                
                {/* Post Engagement */}
                <div className="flex items-center gap-4 px-4 py-3 bg-slate-50 dark:bg-slate-700/50 border-t border-slate-200 dark:border-slate-700">
                  <div className="flex items-center gap-1">
                    <i className="fas fa-thumbs-up text-blue-500 text-sm"></i>
                    <span className="text-sm text-slate-600 dark:text-slate-400">{post.likes || 0}</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <i className="fas fa-comment text-green-500 text-sm"></i>
                    <span className="text-sm text-slate-600 dark:text-slate-400">{post.comments || 0}</span>
                  </div>
                  {post.shares > 0 && (
                    <div className="flex items-center gap-1">
                      <i className="fas fa-share text-purple-500 text-sm"></i>
                      <span className="text-sm text-slate-600 dark:text-slate-400">{post.shares}</span>
                    </div>
                  )}
                </div>
                
                {/* Post Actions */}
                <div className="flex border-t border-slate-200 dark:border-slate-700">
                  <button 
                    onClick={() => toggleComments(post.id)}
                    className="flex-1 flex items-center justify-center gap-2 py-3 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors"
                  >
                    <i className="fas fa-comment text-sm"></i>
                    <span className="text-sm">Comments</span>
                    {commentsVisible[post.id] && <i className="fas fa-chevron-up text-xs"></i>}
                    {!commentsVisible[post.id] && <i className="fas fa-chevron-down text-xs"></i>}
                  </button>
                  <div className="w-px bg-slate-200 dark:border-slate-700"></div>
                  <button className="flex-1 flex items-center justify-center gap-2 py-3 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors">
                    <i className="fas fa-chart-line text-sm"></i>
                    <span className="text-sm">Analytics</span>
                  </button>
                  <div className="w-px bg-slate-200 dark:border-slate-700"></div>
                  <button
                    onClick={() => handleDeletePost(post)}
                    className="flex-1 flex items-center justify-center gap-2 py-3 text-slate-600 dark:text-slate-400 hover:bg-red-50 dark:hover:bg-red-900/20 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                  >
                    <i className="fas fa-trash text-sm"></i>
                    <span className="text-sm">Delete</span>
                  </button>
                </div>

                {/* Comments Section */}
                {commentsVisible[post.id] && (
                  <div className="border-t border-slate-200 dark:border-slate-700">
                    {/* Comments List */}
                    <div className="max-h-96 overflow-y-auto">
                      {loadingComments[post.id] ? (
                        <div className="p-4 text-center">
                          <i className="fas fa-spinner fa-spin text-slate-400"></i>
                          <p className="text-sm text-slate-500 mt-2">Loading comments...</p>
                        </div>
                      ) : comments[post.id] && comments[post.id].length > 0 ? (
                        comments[post.id].map((comment) => (
                          <div key={comment.id} className="p-4 border-b border-slate-100 dark:border-slate-700 last:border-b-0">
                            <div className="flex items-start gap-3">
                              <div className="flex-1">
                                <div className="flex items-center gap-2 mb-1">
                                  <span className="font-medium text-slate-800 dark:text-slate-200 text-sm">
                                    {comment.from.name}
                                  </span>
                                  <span className="text-xs text-slate-500">
                                    {new Date(comment.created_time).toLocaleDateString()}
                                  </span>
                                </div>
                                {editingComment === comment.id ? (
                                  <div className="space-y-2">
                                    <textarea
                                      value={editCommentText}
                                      onChange={(e) => setEditCommentText(e.target.value)}
                                      className="w-full p-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 resize-none"
                                      rows="2"
                                    />
                                    <div className="flex gap-2">
                                      <button
                                        onClick={() => editComment(comment.id)}
                                        className="px-3 py-1 bg-blue-500 text-white rounded text-xs hover:bg-blue-600"
                                      >
                                        Save
                                      </button>
                                      <button
                                        onClick={() => {
                                          setEditingComment(null);
                                          setEditCommentText('');
                                        }}
                                        className="px-3 py-1 bg-slate-500 text-white rounded text-xs hover:bg-slate-600"
                                      >
                                        Cancel
                                      </button>
                                    </div>
                                  </div>
                                ) : (
                                  <div>
                                    <p className="text-slate-700 dark:text-slate-300 text-sm mb-2">
                                      {comment.message}
                                    </p>
                                    <button
                                      onClick={() => {
                                        setReplyingToComment(comment.id);
                                        setReplyText('');
                                      }}
                                      className="text-xs text-slate-500 hover:text-blue-500 transition-colors"
                                    >
                                      Reply
                                    </button>
                                  </div>
                                )}
                              </div>
                              {!editingComment && (
                                <div className="flex gap-1">
                                  <button
                                    onClick={() => {
                                      setEditingComment(comment.id);
                                      setEditCommentText(comment.message);
                                    }}
                                    className="p-1 text-slate-400 hover:text-blue-500 transition-colors"
                                  >
                                    <i className="fas fa-edit text-xs"></i>
                                  </button>
                                  <button
                                    onClick={() => deleteComment(comment.id)}
                                    className="p-1 text-slate-400 hover:text-red-500 transition-colors"
                                  >
                                    <i className="fas fa-trash text-xs"></i>
                                  </button>
                                </div>
                              )}
                            </div>
                            
                            {/* Reply Interface */}
                            {replyingToComment === comment.id && (
                              <div className="mt-3 ml-6 p-3 bg-slate-100 dark:bg-slate-600 rounded-lg">
                                <div className="flex gap-2">
                                  <input
                                    type="text"
                                    placeholder="Write a reply..."
                                    value={replyText}
                                    onChange={(e) => setReplyText(e.target.value)}
                                    onKeyPress={(e) => e.key === 'Enter' && replyToComment(comment.id)}
                                    className="flex-1 px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                                  />
                                  <button
                                    onClick={() => replyToComment(comment.id)}
                                    disabled={!replyText.trim()}
                                    className="px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                                  >
                                    Reply
                                  </button>
                                  <button
                                    onClick={() => {
                                      setReplyingToComment(null);
                                      setReplyText('');
                                    }}
                                    className="px-4 py-2 bg-slate-500 text-white rounded-md hover:bg-slate-600 text-sm"
                                  >
                                    Cancel
                                  </button>
                                </div>
                              </div>
                            )}
                            
                            {/* Display Replies */}
                            {comment.replies && comment.replies.length > 0 && (
                              <div className="mt-3 ml-6">
                                {comment.replies.map((reply) => (
                                  <div key={reply.id} className="p-3 bg-slate-50 dark:bg-slate-700 rounded-lg mb-2">
                                    <div className="flex items-center gap-2 mb-1">
                                      <span className="font-medium text-slate-800 dark:text-slate-200 text-sm">
                                        {reply.from.name}
                                      </span>
                                      <span className="text-xs text-slate-500">
                                        {new Date(reply.created_time).toLocaleDateString()}
                                      </span>
                                    </div>
                                    <p className="text-slate-700 dark:text-slate-300 text-sm">
                                      {reply.message}
                                    </p>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        ))
                      ) : (
                        <div className="p-4 text-center text-slate-500 dark:text-slate-400">
                          <p className="text-sm">No comments yet</p>
                        </div>
                      )}
                    </div>

                    {/* Add Comment */}
                    <div className="p-4 bg-slate-50 dark:bg-slate-700/50">
                      <div className="flex gap-2">
                        <input
                          type="text"
                          placeholder="Add a comment..."
                          value={newComment[post.id] || ''}
                          onChange={(e) => setNewComment(prev => ({ ...prev, [post.id]: e.target.value }))}
                          onKeyPress={(e) => e.key === 'Enter' && addComment(post.id)}
                          className="flex-1 px-3 py-2 border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                        />
                        <button
                          onClick={() => addComment(post.id)}
                          disabled={!newComment[post.id]?.trim()}
                          className="px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                        >
                          Post
                        </button>
                      </div>
                    </div>
                  </div>
                )}
              </div>
              ))}
            </div>

            {hasMorePosts && (
              <div className="text-center mt-8">
                <button
                  onClick={loadMorePosts}
                  disabled={loadingMore}
                  className="inline-flex items-center gap-2 px-6 py-3 bg-white dark:bg-slate-800 border border-slate-300 dark:border-slate-600 hover:border-slate-400 dark:hover:border-slate-500 text-slate-700 dark:text-slate-300 hover:text-slate-900 dark:hover:text-slate-100 rounded-lg transition-all disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {loadingMore ? (
                    <>
                      <i className="fas fa-spinner fa-spin text-sm"></i>
                      Loading more posts...
                    </>
                  ) : (
                    <>
                      <i className="fas fa-chevron-down text-sm"></i>
                      Load More Posts
                    </>
                  )}
                </button>
              </div>
            )}
          </>
        ) : (
          <div className="text-center py-16">
            <div className="w-20 h-20 mx-auto mb-6 flex items-center justify-center bg-slate-100 dark:bg-slate-700 rounded-full">
              <i className="fas fa-file-alt text-2xl text-slate-400 dark:text-slate-500"></i>
            </div>
            <h3 className="text-xl font-medium text-slate-900 dark:text-slate-100 mb-2">No posts found</h3>
            <p className="text-slate-600 dark:text-slate-400 mb-6 max-w-md mx-auto">
              This page doesn't have any recent posts, or we couldn't load them.
            </p>
            <button 
              onClick={() => setPostComposerOpen(true)} 
              className="inline-flex items-center gap-2 px-6 py-3 bg-gradient-to-r from-green-500 to-teal-500 hover:from-green-600 hover:to-teal-600 text-white font-medium rounded-lg transition-all"
            >
              <i className="fas fa-plus text-sm"></i>
              Create First Post
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

export default ContentManager;