#!/usr/bin/env bash
# Creates the 5 logical databases + matching roles for local/dev use, per
# docs/03-system-architecture.md §1 ("one physical instance, five logical
# databases"). Runs automatically via the postgres image's
# /docker-entrypoint-initdb.d/ mechanism on first container start.
set -euo pipefail

for svc in identity core payment notification reporting; do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    CREATE ROLE ${svc} LOGIN PASSWORD '${svc}';
    CREATE DATABASE ${svc}_db OWNER ${svc};
    GRANT ALL PRIVILEGES ON DATABASE ${svc}_db TO ${svc};
EOSQL
done
