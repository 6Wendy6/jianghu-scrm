# AI SCRM Go Backend

This backend is the first API implementation for the offline-store private-domain MVP prototype. It uses in-memory state so frontend modules can connect to real business actions before database modeling is finalized.

## Run

```bash
go run .
```

Default address:

```text
http://127.0.0.1:8080
```

Use another port when 8080 is occupied:

```bash
SCRM_API_ADDR=127.0.0.1:18080 go run .
```

## Main API Groups

- `GET /api/bootstrap`: full page data for the prototype.
- `GET /api/summary`: dashboard metrics.
- `GET /api/stores`, `PATCH /api/stores/{id}/config`: store list and private-domain config.
- `GET/POST /api/stores/{id}/guides`: store guide QR lifecycle.
- `POST /api/guides/{id}/pause`, `POST /api/guides/{id}/remove`, `GET /api/guides/{id}/lifecycle`: guide lifecycle actions.
- `POST /api/guides/{id}/qr-download`: return generated QR SVG payload.
- `POST /api/stores/{id}/handover/sync`, `POST /api/stores/{id}/handover/submit`: Enterprise WeChat leave/transfer handover loop.
- `GET/POST /api/customers`: customer pool.
- `POST /api/customers/batch-tags`: batch system tagging.
- `POST /api/customers/{id}/touch`, `/followups`, `/lead`, `/order`: customer touch and conversion loop.
- `POST /api/customers/{id}/relations`, `/relations/{relationId}/set-main`, `/relations/{relationId}/end`: customer-guide relationship management.
- `GET/POST /api/touches`: welcome, mass-send, SOP, and material rules.
- `GET/POST /api/groups`: customer group files.
- `GET/POST /api/groups/mass`, `/welcomes`, `/sops`, `/calendar`, `/reminders`, `/tag-groups`: customer group operation subpages.
- `GET/POST /api/tags`, `PATCH /api/tags/{name}/toggle`, `PATCH /api/tags/{name}/rename`: tag lifecycle.
- `GET/POST /api/tag-groups`: tag group governance.
- `GET/POST /api/tag-rules/auto`, `/api/tag-rules/pre`: automatic and pre-tag rules.

## Current Boundary

- Data is process-local and resets on restart.
- Auth, organization permission, Enterprise WeChat callbacks, and database persistence are intentionally left for the next backend iteration.
