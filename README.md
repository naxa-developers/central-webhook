# ODK Webhook

Call a remote API on ODK Central database events:

- New submission.
- Update entity .

## Usage

From command line:

```bash
odkhook \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -webhook 'https://your.domain.com/some/webhook'
```

> [!TIP]
> By default both Entity editing and new submissions trigger the webhook.
>
> Use the -trigger flag to modify this behaviour.

From code:

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
    "https://your.domain.com/some/webhook",
    map[string]bool{
        "entity.update.version": true,
        "submission.create":     true,
    },
)
if err != nil {
    fmt.Fprintf(os.Stderr, "error setting up webhook: %v", err)
}
```
