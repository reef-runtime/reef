ALTER TABLE job
ADD COLUMN wasm_id VARCHAR(64) DEFAULT 'invalid' NOT NULL;