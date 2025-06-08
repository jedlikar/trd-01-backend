CREATE DATABASE {db-name};
CREATE USER {db-user} WITH ENCRYPTED PASSWORD {db-password};
GRANT ALL PRIVILEGES ON DATABASE {db-name} TO {db-user};
GRANT CREATE ON SCHEMA public TO {db-user};
GRANT USAGE ON SCHEMA public TO {db-user};
ALTER SCHEMA public OWNER TO {db-user};

CREATE TABLE signal (
                        id SERIAL PRIMARY KEY,
                        data TEXT NOT NULL,
                        ip_address TEXT,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);