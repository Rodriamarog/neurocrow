# Social Dashboard

A Go-based web application that provides a unified dashboard for managing social media messages from Facebook and Instagram platforms.

## Features

- Real-time message management interface
- Unified inbox for Facebook and Instagram messages
- Thread-based conversation view
- Message search functionality
- Platform-specific message indicators
- Responsive design for mobile and desktop

## Tech Stack

- **Backend**: Go 1.22+
- **Frontend**: 
  - HTMX for dynamic interactions
  - Hyperscript for client-side behaviors
  - TailwindCSS for styling
- **Database**: PostgreSQL

## Prerequisites

- Go 1.22 or higher
- PostgreSQL database
- Environment variables configured in `.env` file

## Environment Setup

Create a `.env` file in the root directory with the following variables:

```env
DATABASE_URL=postgresql://username:password@localhost:5432/dbname
PORT=8080 # Optional, defaults to 8080
