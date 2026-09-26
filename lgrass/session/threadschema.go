package session

const threadSchema = `
CREATE TABLE IF NOT EXISTS lg_tabs (
	project_id TEXT NOT NULL,
	tab_id TEXT NOT NULL,
	vendor TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project_id, tab_id)
);
CREATE TABLE IF NOT EXISTS lg_threads (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	title TEXT NOT NULL,
	group_id INTEGER,
	created_by TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_lg_threads_group ON lg_threads(group_id) WHERE group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_lg_threads_project ON lg_threads(project_id, id);
CREATE TABLE IF NOT EXISTS lg_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	thread_id INTEGER NOT NULL,
	tab_id TEXT NOT NULL,
	body TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_lg_messages_thread ON lg_messages(thread_id, id);
CREATE TABLE IF NOT EXISTS lg_groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	pilot_tab_id TEXT NOT NULL,
	created_at TEXT NOT NULL,
	disbanded_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_lg_groups_project ON lg_groups(project_id, disbanded_at);
CREATE TABLE IF NOT EXISTS lg_group_members (
	group_id INTEGER NOT NULL,
	tab_id TEXT NOT NULL,
	role TEXT NOT NULL,
	label TEXT NOT NULL,
	vendor TEXT NOT NULL,
	PRIMARY KEY (group_id, tab_id)
);
CREATE INDEX IF NOT EXISTS idx_lg_group_members_tab ON lg_group_members(tab_id);
CREATE TABLE IF NOT EXISTS lg_ready_marks (
	project_id TEXT NOT NULL,
	tab_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	marked_at TEXT NOT NULL,
	PRIMARY KEY (project_id, tab_id, kind)
);
CREATE TABLE IF NOT EXISTS lg_listener_heartbeats (
	project_id TEXT NOT NULL,
	tab_id TEXT NOT NULL,
	seen_at TEXT NOT NULL,
	PRIMARY KEY (project_id, tab_id)
);
`

const legacyThreadDrop = `
DROP TABLE IF EXISTS thread_messages;
DROP TABLE IF EXISTS thread_participants;
`
