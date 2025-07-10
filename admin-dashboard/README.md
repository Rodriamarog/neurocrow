# Social Media Admin Dashboard

A modern, real-time admin dashboard for managing social media messages across Facebook and Instagram platforms, built with Go.

## 🌟 Features

- **Authentication & Authorization**
  - JWT-based authentication
  - Role-based access control
  - Secure session management

- **Real-time Message Management**
  - Live message updates
  - Thread-based conversation view
  - Message search and filtering
  - Profile picture integration with Meta API
  - Support for both Facebook and Instagram messages

- **Performance Optimizations**
  - In-memory caching
  - Rate limiting
  - Pagination support
  - Efficient database queries

- **Modern UI**
  - Responsive design with Tailwind CSS
  - HTMX for dynamic updates
  - Alpine.js for interactive components
  - Custom scrollbar styling

## 🛠 Tech Stack

- **Backend**
  - Go 1.22.2
  - PostgreSQL
  - JWT for authentication
  - Supabase Realtime (planned)

- **Frontend**
  - HTMX
  - Alpine.js
  - Tailwind CSS
  - Hyperscript

- **External Services**
  - Meta Graph API
  - Facebook Messenger Platform API

## 📋 Prerequisites

- Go 1.22.2 or higher
- PostgreSQL database
- Meta Developer Account (for Facebook/Instagram integration)
- Environment variables configured

## 🚀 Getting Started

1. **Clone the repository**
   ```bash
   git clone [repository-url]
   cd admin-dashboard
   ```

2. **Set up environment variables**
   Create a `.env` file in the root directory:
   ```env
   DATABASE_URL=postgresql://user:password@localhost:5432/dbname
   JWT_SECRET=your-secret-key
   META_API_KEY=your-meta-api-key
   PORT=8080
   ```

3. **Install dependencies**
   ```bash
   go mod download
   ```

4. **Run the application**
   ```bash
   go run main.go
   ```

## 🗄️ Project Structure
