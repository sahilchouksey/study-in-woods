-- Initialize database for Study in Woods
-- This script runs automatically when PostgreSQL container starts

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enable pgcrypto for encryption functions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create initial database (if not exists)
SELECT 'Database initialized successfully' AS status;
