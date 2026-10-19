-- Synthetic v0.2 schema and records, independent of the current initializer.
-- Payloads are deliberately opaque binary data, not real encrypted user data.
CREATE TABLE sync_entities (
    client_id TEXT NOT NULL,
    id TEXT NOT NULL,
    parent_id TEXT,
    version INTEGER,
    mtime INTEGER,
    ctime INTEGER,
    name TEXT,
    non_unique_name TEXT,
    server_defined_unique_tag TEXT,
    deleted INTEGER,
    originator_cache_guid TEXT,
    originator_client_item_id TEXT,
    specifics BLOB,
    data_type INTEGER,
    folder INTEGER,
    client_defined_unique_tag TEXT,
    unique_position BLOB,
    data_type_mtime TEXT,
    expiration_time INTEGER,
    PRIMARY KEY (client_id, id)
);
CREATE INDEX idx_sync_entities_updates ON sync_entities(client_id, data_type, mtime);
CREATE TABLE unique_tags (
    client_id TEXT NOT NULL,
    id TEXT NOT NULL,
    mtime INTEGER,
    ctime INTEGER,
    PRIMARY KEY (client_id, id)
);
CREATE TABLE client_item_counts (
    client_id TEXT PRIMARY KEY,
    item_count INTEGER NOT NULL DEFAULT 0,
    history_item_count_period1 INTEGER NOT NULL DEFAULT 0,
    history_item_count_period2 INTEGER NOT NULL DEFAULT 0,
    history_item_count_period3 INTEGER NOT NULL DEFAULT 0,
    history_item_count_period4 INTEGER NOT NULL DEFAULT 0,
    last_period_change_time INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 0
);

INSERT INTO sync_entities VALUES
    ('legacy-active', 'full-record', 'parent-0', 7, 100, 90,
     'synthetic name', '', 'legacy-root', 0, 'synthetic-device', 'client-item-0',
     X'00FF80A5017F000D0A', 32904, 0, 'legacy-tag', X'FF00807F', '32904#100', 0),
    ('legacy-active', 'nullable-record', NULL, 1, 200, NULL,
     NULL, NULL, NULL, NULL, NULL, NULL, NULL, 32904, NULL, NULL, NULL, NULL, NULL),
    ('legacy-active', 'tombstone', 'parent-0', 3, 300, 80,
     'deleted synthetic item', NULL, NULL, 1, NULL, NULL,
     X'', 32904, 0, 'released-tag', NULL, '32904#300', NULL),
    ('legacy-active', 'expired-history', NULL, 1, 400, 400,
     NULL, NULL, NULL, 0, NULL, NULL, X'00FF', 963985, 0,
     'history-old', NULL, '963985#400', 1),
    ('legacy-active', 'live-history', NULL, 2, 500, 400,
     NULL, NULL, NULL, 0, NULL, NULL, X'80FF00', 963985, 0,
     'history-live', NULL, '963985#500', 4102444800),
    ('legacy-disabled', 'full-record', '', 9, 600, 500,
     'other chain', NULL, NULL, 0, NULL, NULL, X'A500FF', 32904, 1,
     'legacy-tag', X'0080', '32904#600', NULL),
    ('legacy-disabled', 'disabled_chain', NULL, NULL, 700, 700,
     'deleted', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO unique_tags VALUES
    ('legacy-active', 'Client#legacy-tag', 100, 90),
    ('legacy-active', 'Server#legacy-root', 100, 90),
    ('legacy-disabled', 'Client#legacy-tag', 600, 500);
-- A fixed future period anchor prevents wall-clock history rotation in this fixture.
INSERT INTO client_item_counts VALUES
    ('legacy-active', 2, 3, 4, 5, 6, 4102444800, 2),
    ('legacy-disabled', 1, 0, 0, 0, 0, 4102444800, 2);
