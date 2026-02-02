-- Store plugin enrichment data for interactions
CREATE TABLE interaction_attributes (
    interaction_id INTEGER NOT NULL,
    key            TEXT NOT NULL,
    value          TEXT NOT NULL,
    PRIMARY KEY (interaction_id, key),
    FOREIGN KEY (interaction_id) REFERENCES interactions(id) ON DELETE CASCADE
);

CREATE INDEX idx_interaction_attributes_key ON interaction_attributes(key);

-- Store per-token plugin configuration
CREATE TABLE token_plugin_config (
    token_id  INTEGER NOT NULL,
    plugin_id TEXT NOT NULL,
    config    TEXT NOT NULL,
    PRIMARY KEY (token_id, plugin_id),
    FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
);
