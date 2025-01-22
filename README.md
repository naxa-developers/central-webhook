# ODK Webhook

Call a remote API on ODK Central database events:

- New submission.
- Update entity .

## Usage

The `odkhook` tool is a service that runs continually, monitoring the
ODK Central database for updates and triggering the webhook as appropriate.

### Binary

Download the binary for your platform from the
[releases](https://github.com/hotosm/odk-webhook/releases) page.

Then run with:

```bash
./odkhook \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -entityUrl 'https://your.domain.com/some/webhook' \
    -submissionUrl 'https://your.domain.com/some/webhook'
```

> [!TIP]
> It's possible to specify a webhook for only Entities or Submissions, or both.

### Docker

```bash
docker run -d ghcr.io/hotosm/odk-webhook:latest \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -entityUrl 'https://your.domain.com/some/webhook' \
    -submissionUrl 'https://your.domain.com/some/webhook'
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

	"github.com/hotosm/odk-webhook/db"
	"github.com/hotosm/odk-webhook/webhook"
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
