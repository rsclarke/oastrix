CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL
);

CREATE TABLE api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key_prefix TEXT NOT NULL UNIQUE,
  key_hash BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  revoked_at INTEGER
);

CREATE TABLE tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token TEXT NOT NULL UNIQUE,
  api_key_id INTEGER,
  created_at INTEGER NOT NULL,
  label TEXT,
  FOREIGN KEY(api_key_id) REFERENCES api_keys(id)
);

CREATE TABLE interactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token_id INTEGER NOT NULL,
  kind TEXT NOT NULL,
  occurred_at INTEGER NOT NULL,
  remote_ip TEXT NOT NULL,
  remote_port INTEGER,
  tls INTEGER DEFAULT 0,
  summary TEXT,
  FOREIGN KEY(token_id) REFERENCES tokens(id) ON DELETE CASCADE
);

CREATE INDEX idx_interactions_token_time ON interactions(token_id, occurred_at DESC);

CREATE TABLE http_interactions (
  interaction_id INTEGER PRIMARY KEY,
  method TEXT NOT NULL,
  scheme TEXT,
  host TEXT,
  path TEXT,
  query TEXT,
  http_version TEXT,
  request_headers TEXT,
  request_body BLOB,
  FOREIGN KEY(interaction_id) REFERENCES interactions(id) ON DELETE CASCADE
);

CREATE TABLE dns_interactions (
  interaction_id INTEGER PRIMARY KEY,
  qname TEXT NOT NULL,
  qtype INTEGER NOT NULL,
  qclass INTEGER NOT NULL,
  rd INTEGER,
  opcode INTEGER,
  dns_id INTEGER,
  protocol TEXT,
  FOREIGN KEY(interaction_id) REFERENCES interactions(id) ON DELETE CASCADE
);
