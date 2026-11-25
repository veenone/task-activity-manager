package tamstore

// catalogDDL is the project's fix versions and components, which issues name
// by id in their components and fix_versions columns. Both are keyed by
// profile alone: a profile syncs one project at a time, and a sync replaces
// the synced rows of every project so a profile moved to another project
// stops listing the old one's releases. A draft has a negative id and
// draft = 1, the scheme sprint uses, and a sync leaves it alone.
const catalogDDL = `
CREATE TABLE IF NOT EXISTS project_version (
	profile_id   TEXT NOT NULL,
	project      TEXT NOT NULL DEFAULT '',
	id           INTEGER NOT NULL,
	name         TEXT NOT NULL DEFAULT '',
	description  TEXT NOT NULL DEFAULT '',
	start_date   TEXT NOT NULL DEFAULT '',
	release_date TEXT NOT NULL DEFAULT '',
	released     INTEGER NOT NULL DEFAULT 0,
	archived     INTEGER NOT NULL DEFAULT 0,
	draft        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (profile_id, id)
);
CREATE TABLE IF NOT EXISTS project_component (
	profile_id        TEXT NOT NULL,
	project           TEXT NOT NULL DEFAULT '',
	id                INTEGER NOT NULL,
	name              TEXT NOT NULL DEFAULT '',
	description       TEXT NOT NULL DEFAULT '',
	lead_name         TEXT NOT NULL DEFAULT '',
	lead_display_name TEXT NOT NULL DEFAULT '',
	assignee_type     TEXT NOT NULL DEFAULT '',
	draft             INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (profile_id, id)
);`
