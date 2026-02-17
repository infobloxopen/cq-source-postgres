# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-16

### Added

- Initial release of the CloudQuery PostgreSQL Source Plugin
- Dynamic table discovery from PostgreSQL catalog (`pg_class`, `pg_attribute`, `pg_namespace`)
- Full table sync with Apache Arrow record batching
- CDC (Change Data Capture) via PostgreSQL logical replication using pglogrepl
- Support for 25+ PostgreSQL data types with Arrow type mapping
- Configurable pgx log level (`error`, `warn`, `info`, `debug`, `trace`)
- Configurable `rows_per_record` for Arrow record batch sizing
- Destination table name templating with `{{TABLE}}`, `{{UUID}}`, and date placeholders
- Multi-stage Dockerfile for container deployment (alpine-based, <50MB)
- gRPC server on port 7777 (default)
- GitHub Actions CI/CD workflows (test + release)
- Comprehensive E2E test suite against real PostgreSQL 16
- JSON Schema validation for plugin configuration
