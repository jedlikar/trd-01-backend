CREATE DATABASE {db-name};
CREATE USER {db-user} WITH ENCRYPTED PASSWORD {db-password};
GRANT ALL PRIVILEGES ON DATABASE {db-name} TO {db-user};
GRANT CREATE ON SCHEMA public TO {db-user};
GRANT USAGE ON SCHEMA public TO {db-user};
ALTER SCHEMA public OWNER TO {db-user};

CREATE TABLE signal
(
    id         SERIAL PRIMARY KEY,
    data       TEXT NOT NULL,
    ip_address TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE source_file
(
    id                SERIAL PRIMARY KEY,
    name              TEXT NOT NULL,
    path              TEXT NOT NULL,
    source_ip_address TEXT,
    uploaded_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE market_signal
(
    id              SERIAL PRIMARY KEY,
    source_file_id  INTEGER REFERENCES source_file (id),
    symbol          TEXT             NOT NULL,
    action          TEXT             NOT NULL,
    quantity        DOUBLE PRECISION NOT NULL,
    sectype         TEXT             NOT NULL,
    exchange        TEXT             NOT NULL,
    time_in_force   TEXT             NOT NULL,
    order_type      TEXT             NOT NULL,
    lmt_price       DOUBLE PRECISION,
    order_id        INTEGER,
    basket_tag      TEXT             NOT NULL,
    order_ref       TEXT             NOT NULL,
    account         TEXT,
    aux_price       DOUBLE PRECISION,
    parent_order_id INTEGER,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
