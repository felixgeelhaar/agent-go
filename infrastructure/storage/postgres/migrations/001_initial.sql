-- Initial schema for agent-go storage

-- Runs table stores agent run state
CREATE TABLE IF NOT EXISTS runs (
    id VARCHAR(255) PRIMARY KEY,
    goal TEXT NOT NULL,
    current_state VARCHAR(50) NOT NULL,
    vars JSONB DEFAULT '{}',
    evidence JSONB DEFAULT '[]',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    start_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    end_time TIMESTAMP WITH TIME ZONE,
    result JSONB,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for filtering by status
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);

-- Index for filtering by state
CREATE INDEX IF NOT EXISTS idx_runs_current_state ON runs(current_state);

-- Index for filtering by start_time
CREATE INDEX IF NOT EXISTS idx_runs_start_time ON runs(start_time);

-- Index for filtering by goal pattern (full text search)
CREATE INDEX IF NOT EXISTS idx_runs_goal_gin ON runs USING gin(to_tsvector('english', goal));

-- Events table stores event sourcing events
CREATE TABLE IF NOT EXISTS events (
    id VARCHAR(255) PRIMARY KEY,
    run_id VARCHAR(255) NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    type VARCHAR(100) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    payload JSONB NOT NULL DEFAULT '{}',
    sequence BIGINT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, sequence)
);

-- Index for loading events by run_id
CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id);

-- Index for loading events by sequence
CREATE INDEX IF NOT EXISTS idx_events_sequence ON events(run_id, sequence);

-- Index for filtering events by type
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);

-- Index for filtering events by timestamp
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);

-- Snapshots table for efficient event replay
CREATE TABLE IF NOT EXISTS snapshots (
    id VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    run_id VARCHAR(255) NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    data BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(run_id)
);

-- Index for loading snapshot by run_id
CREATE INDEX IF NOT EXISTS idx_snapshots_run_id ON snapshots(run_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to auto-update updated_at on runs table
DROP TRIGGER IF EXISTS update_runs_updated_at ON runs;
CREATE TRIGGER update_runs_updated_at
    BEFORE UPDATE ON runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
