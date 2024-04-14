CREATE TABLE IF NOT EXISTS banner_versions (
    id SERIAL PRIMARY KEY,
    banner_id INT NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_banner_versions_banner_id
        FOREIGN KEY (banner_id)
        REFERENCES banners(id)
        ON DELETE CASCADE
);