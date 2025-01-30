# Central Webhook

Call a remote API on ODK Central database events:

- New submission (XML).
- Update entity (entity properties).
- Submission review (approved, hasIssues, rejected).

The `centralwebhook` binary is small ~15Mb and only consumes
~5Mb of memory when running.

## Prerequisites

- ODK Central running, connecting to an accessible Postgresql database.
- A POST webhook endpoint on your service API, to call when the selected
  event occurs.

## Usage

The `centralwebhook` tool is a service that runs continually, monitoring the
ODK Central database for updates and triggering the webhook as appropriate.

### Integrate Into [ODK Central](https://github.com/getodk/central) Stack

- It's possible to include this as part of the standard ODK Central docker
  compose stack.
- First add the environment variables to your `.env` file:

    ```dotenv
    CENTRAL_WEBHOOK_UPDATE_ENTITY_URL=https://your.domain.com/some/webhook
    CENTRAL_WEBHOOK_REVIEW_SUBMISSION_URL=https://your.domain.com/some/webhook
    CENTRAL_WEBHOOK_NEW_SUBMISSION_URL=https://your.domain.com/some/webhook
    CENTRAL_WEBHOOK_API_KEY=your_api_key_key
    ```

    > [!TIP]
    > Omit a xxx_URL variable if you do not wish to use that particular webhook.
    >
    > The CENTRAL_WEBHOOK_API_KEY variable is also optional, see the
    > [APIs With Authentication](#apis-with-authentication) section.

- Then extend the docker compose configuration at startup:

```bash
# Starting from the getodk/central code repo
docker compose -f docker-compose.yml -f /path/to/this/repo/compose.webhook.yml up -d
```

### Other Ways To Run

<details>
<summary>Via Docker (Standalone)</summary>

#### Via Docker (Standalone)

```bash
docker run -d ghcr.io/hotosm/central-webhook:latest \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -updateEntityUrl 'https://your.domain.com/some/webhook' \
    -newSubmissionUrl 'https://your.domain.com/some/webhook' \
    -reviewSubmissionUrl 'https://your.domain.com/some/webhook'
```

Environment variables are also supported:

```dotenv
CENTRAL_WEBHOOK_DB_URI=postgresql://user:pass@localhost:5432/db_name?sslmode=disable
CENTRAL_WEBHOOK_UPDATE_ENTITY_URL=https://your.domain.com/some/webhook
CENTRAL_WEBHOOK_REVIEW_SUBMISSION_URL=https://your.domain.com/some/webhook
CENTRAL_WEBHOOK_NEW_SUBMISSION_URL=https://your.domain.com/some/webhook
CENTRAL_WEBHOOK_API_KEY=ksdhfiushfiosehf98e3hrih39r8hy439rh389r3hy983y
CENTRAL_WEBHOOK_LOG_LEVEL=DEBUG
```

</details>

<details>
<summary>Via Binary (Standalone)</summary>

#### Via Binary (Standalone)

Download the binary for your platform from the
[releases](https://github.com/hotosm/central-webhook/releases) page.

Then run with:

```bash
./centralwebhook \
    -db 'postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable' \
    -updateEntityUrl 'https://your.domain.com/some/webhook' \
    -newSubmissionUrl 'https://your.domain.com/some/webhook' \
    -reviewSubmissionUrl 'https://your.domain.com/some/webhook'
```

> It's possible to specify a single webhook event, or multiple.

</details>

<details>
<summary>Via Code</summary>

#### Via Code

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
    nil,
    "https://your.domain.com/some/entity/webhook",
    "https://your.domain.com/some/submission/webhook",
    "https://your.domain.com/some/review/webhook",
)
if err != nil {
    fmt.Fprintf(os.Stderr, "error setting up webhook: %v", err)
}
```

> To not provide a webhook for an event, pass `nil` as the url.

</details>

## Request Payload Examples

### Entity Update (updateEntityUrl)

```json
{
    "type": "entity.update.version",
    "id":"uuid:3c142a0d-37b9-4d37-baf0-e58876428181",
    "data": {
        "entityProperty1": "someStringValue",
        "entityProperty2": "someStringValue",
        "entityProperty3": "someStringValue"
    }
}
```

### New Submission (newSubmissionUrl)

```json
{
    "type": "submission.create",
    "id":"uuid:3c142a0d-37b9-4d37-baf0-e58876428181",
    "data": {"xml":"<?xml version='1.0' encoding='UTF-8' ?><data ...."}
}
```

### Review Submission (reviewSubmissionUrl)

```json
{
    "type":"submission.update",
    "id":"uuid:5ed3b610-a18a-46a2-90a7-8c80c82ebbe9",
    "data": {"reviewState":"hasIssues"}
}
```

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
    -updateEntityUrl 'https://your.domain.com/some/webhook' \
    -apiKey 'ksdhfiushfiosehf98e3hrih39r8hy439rh389r3hy983y'
```

## Example Webhook Server

Here is a minimal FastAPI example for receiving the webhook data:

```python
from typing import Annotated, Optional

from fastapi import (
    Depends,
    Header,
)
from fastapi.exceptions import HTTPException
from pydantic import BaseModel


class OdkCentralWebhookRequest(BaseModel):
    """The POST data from the central webhook service."""

    type: OdkWebhookEvents
    # NOTE we cannot use UUID validation, as Central often passes uuid as 'uuid:xxx-xxx'
    id: str
    # NOTE we use a dict to allow flexible passing of the data based on event type
    data: dict


async def valid_api_token(
    x_api_key: Annotated[Optional[str], Header()] = None,
):
    """Check the API token is present for an active database user.

    A header X-API-Key must be provided in the request.
    """
    # Logic to validate the api key here
    return


@router.post("/webhooks/entity-status")
async def update_entity_status_in_fmtm(
    current_user: Annotated[DbUser, Depends(valid_api_token)],
    odk_event: OdkCentralWebhookRequest,
):
    """Update the status for an Entity in our app db.
    """
    log.debug(f"Webhook called with event ({odk_event.type.value})")

    if odk_event.type == OdkWebhookEvents.UPDATE_ENTITY:
        # insert state into db
    elif odk_event.type == OdkWebhookEvents.REVIEW_SUBMISSION:
        # update entity status in odk to match review state
        pass
    elif odk_event.type == OdkWebhookEvents.NEW_SUBMISSION:
        # unsupported for now
        log.debug(
            "The handling of new submissions via webhook is not implemented yet."
        )
    else:
        msg = f"Webhook was called for an unsupported event type ({odk_event.type.value})"
        log.warning(msg)
        raise HTTPException(status_code=400, detail=msg)
```
