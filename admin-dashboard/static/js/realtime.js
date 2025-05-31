// Add this at the top of the file
console.log("Initializing Supabase Realtime with:", {
  url: supabaseUrl,
  hasKey: !!supabaseKey
});

// Add this at the top of your file, after the console.log for initialization
try {
  // Test the Supabase connection
  supabaseClient.auth.getSession().then(response => {
    console.log("✅ Supabase connection test:", response ? "Success" : "Failed");
  }).catch(error => {
    console.error("❌ Supabase connection error:", error);
  });
} catch (error) {
  console.error("❌ Supabase client initialization error:", error);
}

// Realtime message controller
const messageRealtime = {
  // Keep track of active subscriptions
  threadSubscription: null,
  generalSubscription: null,
  activeThreadId: null,
  
  // Initialize both subscriptions
  init: function() {
    console.log("🔄 Initializing realtime subscriptions");
    this.subscribeToAllMessages();
  },
  
  // Subscribe to all messages (for the message list)
  subscribeToAllMessages: function() {
    console.log("🔄 Setting up general message subscription");
    
    try {
      // Use a different approach with broadcast
      this.generalSubscription = supabaseClient
        .channel('messages')
        .on('broadcast', { event: 'message' }, (payload) => {
          console.log("📨 Broadcast message received:", payload);
          // Handle broadcast message
        })
        .on('postgres_changes', {
          event: '*',
          schema: 'public',
          table: 'messages'
        }, (payload) => {
          console.log("📨 Message list change received:", payload);
          this.handleGeneralMessageChange(payload);
        })
        .subscribe((status) => {
          console.log(`🔌 General subscription status: ${status}`, status);
          
          // If subscription fails, start polling
          if (status === 'CHANNEL_ERROR' || status === 'TIMED_OUT') {
            this.startPolling();
          } else if (status === 'SUBSCRIBED') {
            // Send a test broadcast to verify the channel is working
            this.generalSubscription.send({
              type: 'broadcast',
              event: 'message',
              payload: { message: 'Test broadcast message' }
            });
          }
        });
    } catch (error) {
      console.error("❌ Error setting up general subscription:", error);
      this.startPolling();
    }
  },
  
  // Subscribe to a specific thread's messages (for the chat view)
  subscribeToThread: function(threadId) {
    console.log(`🔄 Subscribing to messages for thread: ${threadId}`);
    
    try {
      // Update active thread ID
      this.activeThreadId = threadId;
      
      // Unsubscribe from previous thread subscription if any
      if (this.threadSubscription) {
        supabaseClient.removeChannel(this.threadSubscription);
      }
      
      this.threadSubscription = supabaseClient
        .channel(`thread_${threadId}`)
        .on('postgres_changes', {
          event: '*',
          schema: 'public',
          table: 'messages',
          filter: `thread_id=eq.${threadId}`
        }, (payload) => {
          console.log("📨 Thread-specific change received:", payload);
          this.handleThreadMessageChange(payload);
        })
        .subscribe((status) => {
          console.log(`🔌 Thread subscription status: ${status}`, status);
        });
    } catch (error) {
      console.error(`❌ Error subscribing to thread ${threadId}:`, error);
    }
  },
  
  // Handle message changes for the message list
  handleGeneralMessageChange: function(payload) {
    console.log("🔍 Processing general message change:", payload);
    
    // Make sure we have a valid payload
    if (!payload || !payload.new || !payload.new.thread_id) {
      console.error("❌ Invalid payload received:", payload);
      return;
    }
    
    if (payload.eventType === 'INSERT' || payload.eventType === 'UPDATE') {
      // Update the thread preview in the message list
      console.log(`🔄 Updating thread preview for: ${payload.new.thread_id}`);
      this.updateThreadPreview(payload.new.thread_id);
    }
  },
  
  // Handle message changes for a specific thread (chat view)
  handleThreadMessageChange: function(payload) {
    console.log("🔍 Processing thread message change:", payload);
    
    // Make sure we have a valid payload
    if (!payload || !payload.new || !payload.new.thread_id) {
      console.error("❌ Invalid payload received:", payload);
      return;
    }
    
    // Only process if we're still viewing this thread
    if (this.activeThreadId !== payload.new.thread_id) {
      console.log(`ℹ️ Ignoring message for thread ${payload.new.thread_id} (currently viewing ${this.activeThreadId})`);
      return;
    }
    
    if (payload.eventType === 'INSERT') {
      // Add the new message to the chat view
      console.log(`➕ Adding new message to chat view for thread: ${payload.new.thread_id}`);
      this.addMessageToChatView(payload.new);
    } else if (payload.eventType === 'UPDATE') {
      // Update an existing message (e.g., read status)
      console.log(`🔄 Updating message in chat view: ${payload.new.id}`);
      this.updateMessageInChatView(payload.new);
    } else if (payload.eventType === 'DELETE') {
      // Remove a message from the chat view
      console.log(`➖ Removing message from chat view: ${payload.old.id}`);
      this.removeMessageFromChatView(payload.old);
    }
  },
  
  // Update a thread preview in the message list
  updateThreadPreview: function(threadId) {
    console.log(`🔄 Updating thread preview for: ${threadId}`);
    
    // Check if we're on the message list page
    const messageList = document.getElementById('message-list-wrapper');
    if (!messageList) {
      console.log("ℹ️ Message list not found, skipping thread preview update");
      return;
    }
    
    // Use HTMX to fetch the updated thread preview
    htmx.ajax('GET', `/thread-preview?thread_id=${threadId}`, {
      target: `#thread-preview-${threadId}`,
      swap: 'outerHTML',
      // If the thread doesn't exist in the list yet, add it to the top
      afterSwap: function(swapElement) {
        // If this is a new thread, move it to the top of the list
        const messageList = document.querySelector('#message-list-wrapper .divide-y');
        if (messageList && swapElement.parentNode === messageList) {
          messageList.prepend(swapElement);
        }
      }
    });
  },
  
  // Add a new message to the chat view
  addMessageToChatView: function(message) {
    console.log(`➕ Adding message to chat view: ${message.id}`);
    
    // Only add if we're viewing this thread
    if (this.activeThreadId !== message.thread_id) return;
    
    // Use HTMX to fetch the new message HTML
    htmx.ajax('GET', `/message-bubble?id=${message.id}`, {
      target: '#messages-content',
      swap: 'beforeend',
      // After adding the message, scroll to the bottom
      afterSwap: function() {
        const container = document.getElementById('messages-container');
        if (container) {
          container.scrollTop = container.scrollHeight;
        }
      }
    });
  },
  
  // Update an existing message in the chat view
  updateMessageInChatView: function(message) {
    console.log(`🔄 Updating message in chat view: ${message.id}`);
    
    // Only update if we're viewing this thread
    if (this.activeThreadId !== message.thread_id) return;
    
    // Update the message with the new content/status
    const messageElement = document.querySelector(`[data-message-id="${message.id}"]`);
    if (messageElement) {
      htmx.ajax('GET', `/message-bubble?id=${message.id}`, {
        target: `[data-message-id="${message.id}"]`,
        swap: 'outerHTML'
      });
    }
  },
  
  // Remove a message from the chat view
  removeMessageFromChatView: function(message) {
    console.log(`➖ Removing message from chat view: ${message.id}`);
    
    // Only remove if we're viewing this thread
    if (this.activeThreadId !== message.thread_id) return;
    
    // Remove the message element
    const messageElement = document.querySelector(`[data-message-id="${message.id}"]`);
    if (messageElement) {
      messageElement.remove();
    }
  },
  
  // Handle thread switching
  switchThread: function(threadId) {
    this.subscribeToThread(threadId);
  },
  
  // Clean up subscriptions
  cleanup: function() {
    if (this.threadSubscription) {
      supabaseClient.removeChannel(this.threadSubscription);
      this.threadSubscription = null;
    }
    
    if (this.generalSubscription) {
      supabaseClient.removeChannel(this.generalSubscription);
      this.generalSubscription = null;
    }
    
    this.activeThreadId = null;
  },
  
  // Add this to the messageRealtime object
  testRealtime: function() {
    console.log("🧪 Testing realtime functionality");
    
    // Get a real thread ID from the UI
    let threadId = this.activeThreadId;
    
    // If we're not in a thread view, try to get the first thread ID from the message list
    if (!threadId) {
      const firstThreadPreview = document.querySelector('[id^="thread-preview-"]');
      if (firstThreadPreview) {
        threadId = firstThreadPreview.id.replace('thread-preview-', '');
      } else {
        console.error("❌ No thread ID found for testing");
        return;
      }
    }
    
    console.log(`🧪 Using thread ID for test: ${threadId}`);
    
    // Simulate a new message event with a real thread ID
    const testPayload = {
      eventType: 'INSERT',
      new: {
        id: 'test-' + Date.now(),
        thread_id: threadId,
        content: 'Test message at ' + new Date().toLocaleTimeString()
      }
    };
    
    console.log("🧪 Simulating message event:", testPayload);
    
    if (this.activeThreadId) {
      this.handleThreadMessageChange(testPayload);
    } else {
      this.handleGeneralMessageChange(testPayload);
    }
  },
  
  // Update the testDirectUpdate function
  testDirectUpdate: function() {
    console.log("🧪 Testing direct UI update");
    
    // Test both message list and chat view updates
    
    // 1. Update the message list (if visible)
    const messageList = document.getElementById('message-list-wrapper');
    if (messageList) {
      // Create a test thread preview
      const testThreadPreview = document.createElement('div');
      testThreadPreview.className = 'p-4 hover:bg-gray-50 active:bg-gray-100 cursor-pointer';
      testThreadPreview.id = 'thread-preview-test-' + Date.now();
      
      const timestamp = new Date().toLocaleTimeString();
      
      testThreadPreview.innerHTML = `
        <div class="flex items-center justify-between mb-1">
          <div class="flex items-center">
            <div class="w-2 h-2 bg-blue-500 rounded-full mr-2"></div>
            <span class="text-sm font-medium text-blue-600">Test Platform</span>
          </div>
          <span class="text-xs text-gray-500">${timestamp}</span>
        </div>
        <div class="text-sm font-medium">Test User</div>
        <div class="text-sm text-gray-600 truncate">Test message created at ${timestamp}</div>
      `;
      
      // Add it to the beginning of the message list
      if (messageList.firstChild) {
        messageList.insertBefore(testThreadPreview, messageList.firstChild);
        console.log("✅ Test thread preview added to message list");
      } else {
        messageList.appendChild(testThreadPreview);
        console.log("✅ Test thread preview added to empty message list");
      }
    }
    
    // 2. Update the chat view (if visible)
    const messagesContent = document.getElementById('messages-content');
    if (messagesContent) {
      // Create a test message element
      const testMessage = document.createElement('div');
      testMessage.className = 'message-bubble mb-2';
      testMessage.setAttribute('data-message-id', 'test-' + Date.now());
      
      // Create the message content
      testMessage.innerHTML = `
        <div class="flex items-start max-w-[85%] justify-end ml-auto space-x-2">
          <div class="bg-indigo-600 text-white rounded-lg px-4 py-2 break-words max-w-full relative group">
            <p class="text-sm break-words">Test message created at ${new Date().toLocaleTimeString()}</p>
            <span class="absolute -bottom-4 right-0 text-xs text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
              ${new Date().toLocaleTimeString()}
            </span>
          </div>
        </div>
      `;
      
      // Add it to the messages container
      messagesContent.appendChild(testMessage);
      
      // Scroll to the bottom
      const container = document.getElementById('messages-container');
      if (container) {
        container.scrollTop = container.scrollHeight;
      }
      
      console.log("✅ Test message added to chat view");
    } else {
      console.log("ℹ️ Chat view not visible, skipping direct chat update");
    }
  },
  
  // Add this to the messageRealtime object
  startPolling: function() {
    // Don't start polling if it's already running
    if (this.pollingInterval) {
      return;
    }
    
    console.log("🔄 Starting polling fallback due to WebSocket failure");
    
    // Start with an immediate refresh
    this.pollForUpdates();
    
    // Then set up the interval
    this.pollingInterval = setInterval(() => {
      this.pollForUpdates();
    }, 5000); // Poll every 5 seconds
  },
  
  pollForUpdates: function() {
    // If we're in a thread view, refresh the messages
    if (this.activeThreadId) {
      console.log("🔄 Polling for new messages in thread:", this.activeThreadId);
      
      fetch(`/chat-messages?thread_id=${this.activeThreadId}`)
        .then(response => {
          if (response.ok) {
            return response.text();
          }
          throw new Error(`Failed to fetch chat messages: ${response.status}`);
        })
        .then(html => {
          const messagesContent = document.getElementById('messages-content');
          if (messagesContent) {
            messagesContent.innerHTML = html;
            
            // Scroll to bottom
            const container = document.getElementById('messages-container');
            if (container) {
              container.scrollTop = container.scrollHeight;
            }
          }
        })
        .catch(error => {
          console.error("❌ Error polling chat messages:", error);
        });
    }
    
    // Always refresh the message list
    console.log("🔄 Polling for updated thread list");
    fetch('/message-list')
      .then(response => {
        if (response.ok) {
          return response.text();
        }
        throw new Error(`Failed to fetch message list: ${response.status}`);
      })
      .then(html => {
        const messageList = document.getElementById('message-list-wrapper');
        if (messageList) {
          messageList.innerHTML = html;
        }
      })
      .catch(error => {
        console.error("❌ Error polling message list:", error);
      });
  },
  
  stopPolling: function() {
    if (this.pollingInterval) {
      console.log("🛑 Stopping polling fallback");
      clearInterval(this.pollingInterval);
      this.pollingInterval = null;
    }
  }
};

// Initialize realtime subscriptions when page loads
document.addEventListener('DOMContentLoaded', function() {
  messageRealtime.init();
});

// HTMX event listeners
document.addEventListener('htmx:afterOnLoad', function(event) {
  // If chat view is loaded
  if (event.detail.target.id === 'chat-view' && !event.detail.target.classList.contains('hidden')) {
    // Extract thread ID from the URL
    const urlParams = new URLSearchParams(window.location.search);
    const threadId = urlParams.get('thread_id');
    
    if (threadId) {
      // Subscribe to realtime updates for this specific thread
      messageRealtime.switchThread(threadId);
    }
  }
});

// Clean up thread subscription when navigating away from chat
document.body.addEventListener('click', function(event) {
  if (event.target.classList.contains('back-button')) {
    messageRealtime.activeThreadId = null;
    if (messageRealtime.threadSubscription) {
      supabaseClient.removeChannel(messageRealtime.threadSubscription);
      messageRealtime.threadSubscription = null;
    }
  }
});

// Handle page unload
window.addEventListener('beforeunload', function() {
  messageRealtime.cleanup();
}); 