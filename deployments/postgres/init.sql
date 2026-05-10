-- PostgreSQL initialization script
-- Mounted to /docker-entrypoint-initdb.d/ and runs on first container start.
-- Creates the auth, finance, and datarights schemas used by their respective services.

CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS finance;
CREATE SCHEMA IF NOT EXISTS datarights;
