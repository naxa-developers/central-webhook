# Central Webhook

Call a remote API on ODK Central database events:

- New submission.
- Update entity.

## Usage

The `centralwebhook` tool is a service that runs continually, monitoring the
ODK Central database for updates and triggering the webhook as appropriate.

### Binary

Download the binary for your platform from the
[releases](https://github.com/hotosm/central-webhook/releases) page.

Then run with:

```bash
./centralwebhook \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -entityUrl 'https://your.domain.com/some/webhook' \
    -submissionUrl 'https://your.domain.com/some/webhook'
```

> [!TIP]
> It's possible to specify a webhook for only Entities or Submissions, or both.

### Docker

```bash
docker run -d ghcr.io/hotosm/central-webhook:latest \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -entityUrl 'https://your.domain.com/some/webhook' \
    -submissionUrl 'https://your.domain.com/some/webhook'
```

Environment variables are also supported:

```dotenv
CENTRAL_WEBHOOK_DB_URI=postgresql://user:pass@localhost:5432/db_name?sslmode=disable
CENTRAL_WEBHOOK_ENTITY_URL=https://your.domain.com/some/webhook
CENTRAL_WEBHOOK_SUBMISSION_URL=https://your.domain.com/some/webhook
CENTRAL_WEBHOOK_API_KEY=ksdhfiushfiosehf98e3hrih39r8hy439rh389r3hy983y
CENTRAL_WEBHOOK_LOG_LEVEL=DEBUG
```

> [!NOTE]
> Alternatively, add the service to your docker compose stack.

### Code

Usage via the code / API:

```go
package main

import (
    "fmt"
    "context"
    "log/slog"

	"github.com/hotosm/central-webhook/db"
	"github.com/hotosm/central-webhook/webhook"
)

ctx := context.Background()
log := slog.New()

dbPool, err := db.InitPool(ctx, log, "postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable")
if err != nil {
    fmt.Fprintf(os.Stderr, "could not connect to database: %v", err)
}

err = SetupWebhook(
    log,
    ctx,
    dbPool,
    "https://your.domain.com/some/entity/webhook",
    "https://your.domain.com/some/submission/webhook",
)
if err != nil {
    fmt.Fprintf(os.Stderr, "error setting up webhook: %v", err)
}
```

> [!NOTE]
> To not provide a webhook for either entities or submissions,
> pass `nil` instead.

## APIs With Authentication

Many APIs will not be public and require some sort of authentication.

There is an optional `-apiKey` flag that can be used to pass
an API key / token provided by the application.

This will be inserted in the `X-API-Key` request header.

No other authentication methods are supported for now, but feel
free to open an issue (or PR!) for a proposal to support other
auth methods.

Example:

```bash
./centralwebhook \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -entityUrl 'https://your.domain.com/some/webhook' \
    -apiKey 'ksdhfiushfiosehf98e3hrih39r8hy439rh389r3hy983y'
```
