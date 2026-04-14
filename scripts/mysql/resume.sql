CREATE TABLE IF NOT EXISTS resumes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    file_name VARCHAR(512) NOT NULL,
    file_url VARCHAR(1024) NOT NULL,
    file_type VARCHAR(32) NOT NULL,
    raw_text TEXT,
    parsed JSONB,
    status SMALLINT DEFAULT 0,
    score JSONB,
    ctime BIGINT NOT NULL,
    utime BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_resumes_user_id ON resumes(user_id);
