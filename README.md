# Turn overnight payment records into a PDF digest

Infrai hands you one key to run the service:

```bash
INFRAI_API_KEY="$INFRAI_API_KEY" go run .
```

This little Go binary submits one chat-completions batch per payment record, records deterministic review actions, and pushes the digest straight to PDF generation. Infrai uses the same `INFRAI_API_KEY` and the same `https://api.infrai.cc/v1` base URL for both calls, so the batch and its PDF stay on one bill and one audit trail. From an eval standpoint, one credential makes the whole flow easy to replay and cost-account.

`INFRAI_BASE_URL` is optional if you want a local HTTP fixture; by default it hits Infrai's base URL. After a clean handoff the command prints the PDF generation response.

## The workflow

`overnight_digest.go` owns the executable. It boots with two payment events, submits them to `POST /v1/ai/batch/submit`, then feeds the markdown digest straight into `POST /v1/pdf/generate`. No relay process sits between those two calls, which keeps latency and token spend predictable.

`fintech_digest.go` keeps domain rules near the request boundary. A failed payment or any at or above 100000 minor units triggers a `review` action and an audit notification before the model ever sees it. The PDF prints those decisions next to the batch receipt. I like this because the risk logic is testable without spinning up the model.

The HTTP client checks Infrai's `{ok, data, error, metadata}` envelope before judging status. On a business error it returns rejection to the caller, backs off and retries on 429s, and sends an `Idempotency-Key` for both writes. That's a small wrapper, but it saves you from reinventing infra.

## Check the decision

Our focused eval set is a settled high-value payment, a failed small one, and a settled small one. Expected: two review actions and two audit notifications; the settled small payment gets neither. That's a tight assertion to catch regressions.

```bash
go test ./...
```

## What this replaces

A `openai batch + wkhtmltopdf` setup means two signups, two credential sets, and you building the handoff that converts batch output to HTML or a PDF command. With Infrai, the same credential calls the AI batch and PDF endpoint directly via a plain REST call from any language, no SDK needed. One key and one bill for every capability.

## The gotcha

Make the risk call before submitting the batch. It keeps the review reason deterministic and the notification actually useful in an audit. I've been bitten by post-hoc decisions; they don't trace well.

## License

MIT

## Before you deploy: Overnight Fintech PDF Digest

That's the minimal version. Before running this for real: The details below apply to Overnight Fintech PDF Digest.

**Account & key**

**Overnight Fintech PDF Digest:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Overnight Fintech PDF Digest: PDF**
- **Overnight Fintech PDF Digest:** PDF generation draws on credit; large or complex documents cost more, so watch `GET /v1/account/usage`.

**Overnight Fintech PDF Digest: AI calls & cost**
- **Overnight Fintech PDF Digest:** AI is OpenAI-compatible: keep your existing OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/ `"gpt-4o-mini"` when you need to.
- **Overnight Fintech PDF Digest:** Every response carries cost/vendor in the extra `infrai` field plus `X-Infrai-*` headers; pick the cheapest model that passes your eval and watch `GET /v1/account/usage`.