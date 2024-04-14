CREATE TABLE IF NOT EXISTS banner_feature_tags (
    id SERIAL PRIMARY KEY,
    banner_id INT NOT NULL,
    feature_id INT NOT NULL,
    tag_id INT NOT NULL,
    CONSTRAINT fk_banner
        FOREIGN KEY (banner_id) REFERENCES banners(id),
    CONSTRAINT fk_feature
        FOREIGN KEY (feature_id) REFERENCES features(id),
    CONSTRAINT fk_tag
        FOREIGN KEY (tag_id) REFERENCES tags(id),
    CONSTRAINT unique_feature_tag
        UNIQUE (feature_id, tag_id)
);