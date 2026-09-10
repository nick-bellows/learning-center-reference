param(
    # Rebuild drifted projection rows. Without it the command only reports (exit 3 on drift).
    [switch]$Apply
)
$ErrorActionPreference = "Stop"

# The projection is a cache of the append-only progress_event log. This runs the
# reconcile command inside the API container as the database owner; the dry run
# is the default so you see what would change before anything is rewritten.
$arguments = @()
if ($Apply) { $arguments += "--apply" }
docker compose exec -T api /reconcileprogress @arguments
exit $LASTEXITCODE
