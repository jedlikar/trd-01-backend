-- CREATE DATABASE trd_db;
-- CREATE USER trd_user WITH ENCRYPTED PASSWORD 'strongpassword123';
GRANT ALL PRIVILEGES ON DATABASE trd_db TO trd_user;

CREATE TABLE signal (
                        id SERIAL PRIMARY KEY,
                        data TEXT NOT NULL,
                        ip_address TEXT,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);