$ErrorActionPreference = "Stop"

# This command is intentionally explicit and scoped to the fixed fictional
# association. Enrollment deletion cascades to its progress events/projection.
docker compose exec -T -e RESET_CONFIRM=synthetic-demo api /resetdemo
