package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// MetricsHistoryService handles persistent storage of connection metrics
type MetricsHistoryService struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// NewMetricsHistoryService creates or opens a metrics history database
func NewMetricsHistoryService(dataDir string) (*MetricsHistoryService, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "metrics.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys and performance settings
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	service := &MetricsHistoryService{
		db:   db,
		path: dbPath,
	}

	// Initialize schema
	if err := service.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return service, nil
}

// initSchema creates the database schema if it doesn't exist
func (s *MetricsHistoryService) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS connection_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		connection_id TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		latency_ms INTEGER,
		bytes_sent INTEGER,
		bytes_received INTEGER,
		uptime_seconds REAL,
		failure_count INTEGER,
		is_healthy BOOLEAN,
		method TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS failover_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		connection_id TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		old_primary_id TEXT,
		new_primary_id TEXT,
		reason TEXT,
		details TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS connection_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		connection_id TEXT NOT NULL,
		start_time INTEGER NOT NULL,
		end_time INTEGER,
		uptime_seconds REAL,
		failure_count INTEGER DEFAULT 0,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Indexes for efficient queries
	CREATE INDEX IF NOT EXISTS idx_metrics_connection_timestamp
		ON connection_metrics(connection_id, timestamp DESC);

	CREATE INDEX IF NOT EXISTS idx_metrics_timestamp
		ON connection_metrics(timestamp DESC);

	CREATE INDEX IF NOT EXISTS idx_failover_timestamp
		ON failover_events(timestamp DESC);

	CREATE INDEX IF NOT EXISTS idx_failover_connection
		ON failover_events(connection_id, timestamp DESC);

	CREATE INDEX IF NOT EXISTS idx_sessions_connection
		ON connection_sessions(connection_id, start_time DESC);
	`

	_, err := s.db.Exec(schema)
	return err
}

// RecordMetrics stores a metrics sample for a connection
func (s *MetricsHistoryService) RecordMetrics(connID, method string, metrics *ConnectionMetrics, isHealthy bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	timestamp := time.Now().Unix()

	_, err := s.db.Exec(`
		INSERT INTO connection_metrics
		(connection_id, timestamp, latency_ms, bytes_sent, bytes_received,
		 uptime_seconds, failure_count, is_healthy, method)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		connID,
		timestamp,
		metrics.Latency.Milliseconds(),
		metrics.BytesSent,
		metrics.BytesReceived,
		metrics.Uptime.Seconds(),
		metrics.FailureCount,
		isHealthy,
		method,
	)

	return err
}

// RecordFailoverEvent logs a failover or primary change event
func (s *MetricsHistoryService) RecordFailoverEvent(eventType, connID, oldPrimaryID, newPrimaryID, reason string, details map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().Unix()

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal details: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO failover_events
		(event_type, connection_id, timestamp, old_primary_id, new_primary_id, reason, details)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		eventType,
		connID,
		timestamp,
		oldPrimaryID,
		newPrimaryID,
		reason,
		string(detailsJSON),
	)

	return err
}

// StartSession records the start of a connection session
func (s *MetricsHistoryService) StartSession(connID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	startTime := time.Now().Unix()

	_, err := s.db.Exec(`
		INSERT INTO connection_sessions
		(connection_id, start_time, status)
		VALUES (?, ?, 'active')
	`,
		connID,
		startTime,
	)

	return err
}

// EndSession marks a connection session as ended
func (s *MetricsHistoryService) EndSession(connID string, failureCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	endTime := time.Now().Unix()

	// Get the start time of the active session
	var startTime int64
	err := s.db.QueryRow(`
		SELECT start_time FROM connection_sessions
		WHERE connection_id = ? AND status = 'active'
		ORDER BY start_time DESC LIMIT 1
	`, connID).Scan(&startTime)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil // No active session to end
		}
		return err
	}

	uptime := float64(endTime - startTime)

	_, err = s.db.Exec(`
		UPDATE connection_sessions
		SET end_time = ?, uptime_seconds = ?, failure_count = ?, status = 'ended'
		WHERE connection_id = ? AND start_time = ? AND status = 'active'
	`,
		endTime,
		uptime,
		failureCount,
		connID,
		startTime,
	)

	return err
}

// GetConnectionMetrics retrieves historical metrics for a connection
func (s *MetricsHistoryService) GetConnectionMetrics(connID string, limit int, since time.Time) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT timestamp, latency_ms, bytes_sent, bytes_received,
			uptime_seconds, failure_count, is_healthy, method
		FROM connection_metrics
		WHERE connection_id = ? AND timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, connID, since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var timestamp int64
		var latencyMs int64
		var bytesSent, bytesReceived int64
		var uptimeSeconds float64
		var failureCount int
		var isHealthy bool
		var method string

		err := rows.Scan(&timestamp, &latencyMs, &bytesSent, &bytesReceived,
			&uptimeSeconds, &failureCount, &isHealthy, &method)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"timestamp":      time.Unix(timestamp, 0).UTC(),
			"latency_ms":     latencyMs,
			"bytes_sent":     bytesSent,
			"bytes_received": bytesReceived,
			"uptime_seconds": uptimeSeconds,
			"failure_count":  failureCount,
			"is_healthy":     isHealthy,
			"method":         method,
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// GetFailoverEvents retrieves failover events within a time range
func (s *MetricsHistoryService) GetFailoverEvents(limit int, since time.Time) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT event_type, connection_id, timestamp, old_primary_id,
			new_primary_id, reason, details
		FROM failover_events
		WHERE timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var eventType, connID, reason string
		var timestamp int64
		var oldPrimaryID, newPrimaryID sql.NullString
		var detailsJSON string

		err := rows.Scan(&eventType, &connID, &timestamp, &oldPrimaryID,
			&newPrimaryID, &reason, &detailsJSON)
		if err != nil {
			return nil, err
		}

		var details map[string]interface{}
		if detailsJSON != "" {
			err = json.Unmarshal([]byte(detailsJSON), &details)
			if err != nil {
				return nil, err
			}
		}

		result := map[string]interface{}{
			"event_type":     eventType,
			"connection_id":  connID,
			"timestamp":      time.Unix(timestamp, 0).UTC(),
			"old_primary_id": oldPrimaryID.String,
			"new_primary_id": newPrimaryID.String,
			"reason":         reason,
			"details":        details,
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// GetConnectionStats calculates aggregate statistics for a connection
func (s *MetricsHistoryService) GetConnectionStats(connID string, since time.Time) (*ConnectionStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &ConnectionStats{
		ConnectionID: connID,
		Since:        since,
	}

	// Get session information
	err := s.db.QueryRow(`
		SELECT
			COUNT(*) as total_sessions,
			COALESCE(SUM(uptime_seconds), 0) as total_uptime,
			COALESCE(SUM(failure_count), 0) as total_failures
		FROM connection_sessions
		WHERE connection_id = ? AND start_time >= ?
	`, connID, since.Unix()).Scan(&stats.TotalSessions, &stats.TotalUptimeSeconds, &stats.TotalFailures)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get average latency from metrics
	err = s.db.QueryRow(`
		SELECT AVG(latency_ms)
		FROM connection_metrics
		WHERE connection_id = ? AND timestamp >= ? AND latency_ms > 0
	`, connID, since.Unix()).Scan(&stats.AvgLatencyMs)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get failover count for this connection
	err = s.db.QueryRow(`
		SELECT COUNT(*)
		FROM failover_events
		WHERE (connection_id = ? OR old_primary_id = ? OR new_primary_id = ?)
			AND timestamp >= ?
	`, connID, connID, connID, since.Unix()).Scan(&stats.FailoverCount)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Calculate MTTR (Mean Time To Recover)
	// MTTR = average downtime between failures and recovery
	err = s.db.QueryRow(`
		SELECT AVG(end_time - start_time)
		FROM connection_sessions
		WHERE connection_id = ? AND start_time >= ?
			AND status = 'ended' AND failure_count > 0
	`, connID, since.Unix()).Scan(&stats.MTTRSeconds)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Calculate uptime percentage
	timeRange := time.Since(since).Seconds()
	if timeRange > 0 {
		stats.UptimePercentage = (stats.TotalUptimeSeconds / timeRange) * 100
	}

	return stats, nil
}

// GetAllConnectionsStats gets stats for all connections since a given time
func (s *MetricsHistoryService) GetAllConnectionsStats(since time.Time) (map[string]*ConnectionStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get all unique connection IDs
	rows, err := s.db.Query(`
		SELECT DISTINCT connection_id FROM connection_metrics
		WHERE timestamp >= ?
		UNION
		SELECT DISTINCT connection_id FROM connection_sessions
		WHERE start_time >= ?
	`, since.Unix(), since.Unix())

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connIDs []string
	for rows.Next() {
		var connID string
		if err := rows.Scan(&connID); err != nil {
			return nil, err
		}
		connIDs = append(connIDs, connID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Get stats for each connection
	result := make(map[string]*ConnectionStats)
	for _, connID := range connIDs {
		stats, err := s.GetConnectionStats(connID, since)
		if err != nil {
			return nil, err
		}
		result[connID] = stats
	}

	return result, nil
}

// ConnectionStats represents aggregate statistics for a connection
type ConnectionStats struct {
	ConnectionID         string
	Since                time.Time
	TotalSessions        int
	TotalUptimeSeconds   float64
	TotalFailures       int
	AvgLatencyMs        float64
	FailoverCount       int
	MTTRSeconds         float64
	UptimePercentage    float64
}

// CleanupOldData removes metrics older than the specified retention period
func (s *MetricsHistoryService) CleanupOldData(retention time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-retention).Unix()

	// Clean up old metrics
	_, err := s.db.Exec("DELETE FROM connection_metrics WHERE timestamp < ?", cutoff)
	if err != nil {
		return err
	}

	// Clean up old failover events (keep these longer for audit trail)
	_, err = s.db.Exec("DELETE FROM failover_events WHERE timestamp < ?", cutoff)
	if err != nil {
		return err
	}

	// Clean up old completed sessions
	_, err = s.db.Exec(`
		DELETE FROM connection_sessions
		WHERE start_time < ? AND status = 'ended'
	`, cutoff)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func (s *MetricsHistoryService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// Vacuum optimizes the database file size
func (s *MetricsHistoryService) Vacuum() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("VACUUM")
	return err
}
