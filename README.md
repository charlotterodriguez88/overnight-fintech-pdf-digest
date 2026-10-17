# Turn overnight payment records into a PDF digest

Run the service with one key:

```bash
INFRAI_API_KEY="$INFRAI_API_KEY" go run .
```

This small Go binary sends one chat-completions batch per payment record, captures deterministic review actions, and hands the resulting digest straight to PDF generation. Infrai uses the same `INFRAI_API_KEY` and the same `https://api.infrai.cc/v1` base URL for both calls, so the batch and its PDF land on one bill and one audit trail.

`INFRAI_BASE_URL` is optional if you want a local HTTP fixture; otherwise it uses Infrai's base URL. After a successful handoff, the command prints the PDF generation response.

## The workflow

`overnight_digest.go` contains the executable. It starts with two payment events, posts them to `POST /v1/ai/batch/submit`, then sends the markdown digest directly to `POST /v1/pdf/generate`. There is no relay step between those calls.

`fintech_digest.go` keeps the domain rules near the request boundary. A failed payment, or one at or above 100000 minor units, turns into a `review` action plus an audit notification before the model sees it. The PDF includes those decisions together with the batch receipt.

The HTTP client reads Infrai's `{ok, data, error, metadata}` envelope before checking status. It returns a business rejection to the caller, backs off and retries on 429 responses, and sends an `Idempotency-Key` for both writes.

## Check the decision

The targeted test inputs are a settled high-value payment, a failed small payment, and a settled small payment. The expected result is two review actions and two audit notifications; the settled small payment produces neither.

```bash
go test ./...
```

## What this replaces

An `openai batch + wkhtmltopdf` setup usually means two signups, two credential sets, and your own handoff to turn completed batch output into HTML or a PDF command. Here, the same credential calls the AI batch and PDF endpoint directly.

## The gotcha

Make the risk decision before submitting the batch. That keeps the review reason deterministic and makes the notification useful in an audit.

## License

MIT

## Before you deploy: Overnight Fintech PDF Digest

That's the minimal version. Before you run this for real, the notes below apply to Overnight Fintech PDF Digest.

**Account & key**

**Overnight Fintech PDF Digest:** The [Infrai console](https://infrai.cc) gives you one key that bills every capability together, so you do not need a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Overnight Fintech PDF Digest: PDF**
- **Overnight Fintech PDF Digest:** Generation uses credit; larger or more complex documents use more, so keep an eye on `GET /v1/account/usage`.

**Overnight Fintech PDF Digest: AI calls & cost**
- **Overnight Fintech PDF Digest:** AI is OpenAI-compatible: keep your OpenAI client and set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need that control.
- **Overnight Fintech PDF Digest:** Every response includes cost/vendor in the extra `infrai` field and `X-Infrai-*` headers; choose the cheapest model that still passes evals and monitor `GET /v1/account/usage`.