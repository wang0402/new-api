# New API Direct Deployment

Docker Compose deployment for New API with Postgres and Redis.

## Usage

1. Copy `.env.example` to `.env`.
2. Replace the placeholder secrets in `.env`.
3. Start the stack:

```sh
docker compose up -d
```

The New API service listens on `127.0.0.1:3001` by default.

## Notes

- `data/` and `logs/` are runtime directories and are intentionally not tracked.
- Database and Redis credentials are loaded from `.env`.
