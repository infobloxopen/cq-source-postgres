-- Seed data for E2E tests
-- Creates tables with all supported PostgreSQL types and populates with test data.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Table with all supported PostgreSQL types
CREATE TABLE IF NOT EXISTS test_types (
    id              SERIAL PRIMARY KEY,
    col_smallint    SMALLINT,
    col_integer     INTEGER,
    col_bigint      BIGINT,
    col_real        REAL,
    col_double      DOUBLE PRECISION,
    col_numeric     NUMERIC(10,2),
    col_boolean     BOOLEAN,
    col_text        TEXT,
    col_varchar     VARCHAR(255),
    col_char        CHAR(10),
    col_bytea       BYTEA,
    col_timestamp   TIMESTAMP,
    col_timestamptz TIMESTAMPTZ,
    col_date        DATE,
    col_time        TIME,
    col_timetz      TIMETZ,
    col_interval    INTERVAL,
    col_uuid        UUID DEFAULT uuid_generate_v4(),
    col_json        JSON,
    col_jsonb       JSONB,
    col_inet        INET,
    col_cidr        CIDR,
    col_macaddr     MACADDR,
    col_xml         XML,
    col_point       POINT,
    col_int_array   INTEGER[],
    col_text_array  TEXT[],
    col_null_col    TEXT
);

-- Simple relational tables for testing joins and references
CREATE TABLE IF NOT EXISTS test_users (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS test_orders (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER REFERENCES test_users(id),
    amount     NUMERIC(10,2),
    status     TEXT DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Empty table to verify empty-result handling
CREATE TABLE IF NOT EXISTS test_empty_table (
    id   SERIAL PRIMARY KEY,
    name TEXT
);

-- Truncate all tables to ensure idempotent seeding
TRUNCATE test_types, test_users, test_orders, test_empty_table RESTART IDENTITY CASCADE;

-- Insert rows with all types populated
INSERT INTO test_types (
    col_smallint, col_integer, col_bigint, col_real, col_double,
    col_numeric, col_boolean, col_text, col_varchar, col_char,
    col_bytea, col_timestamp, col_timestamptz, col_date, col_time,
    col_timetz, col_interval, col_uuid, col_json, col_jsonb,
    col_inet, col_cidr, col_macaddr, col_xml, col_point,
    col_int_array, col_text_array, col_null_col
) VALUES (
    1, 42, 9223372036854775807, 3.14, 2.718281828,
    12345.67, true, 'hello world', 'varchar value', 'charval   ',
    E'\\xDEADBEEF', '2026-01-15 10:30:00', '2026-01-15 10:30:00+00', '2026-01-15', '10:30:00',
    '10:30:00+05', '1 year 2 months', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', '{"key": "value"}', '{"nested": {"key": 42}}',
    '192.168.1.1', '10.0.0.0/8', '08:00:2b:01:02:03', '<root/>', '(1.5,2.5)',
    '{1,2,3}', '{a,b,c}', NULL
);

-- Insert a row with NULLs for most columns
INSERT INTO test_types (col_smallint) VALUES (NULL);

-- Insert edge-case values
INSERT INTO test_types (
    col_smallint, col_integer, col_bigint, col_real, col_double,
    col_numeric, col_boolean, col_text
) VALUES (
    -32768, -2147483648, -9223372036854775808, -3.4e+38, -1.7e+308,
    -99999999.99, false, ''
);

-- Insert users
INSERT INTO test_users (name, email) VALUES
    ('Alice', 'alice@example.com'),
    ('Bob', 'bob@example.com'),
    ('Charlie', NULL);

-- Insert orders
INSERT INTO test_orders (user_id, amount, status) VALUES
    (1, 100.50, 'completed'),
    (1, 200.00, 'pending'),
    (2, 50.25, 'completed');
