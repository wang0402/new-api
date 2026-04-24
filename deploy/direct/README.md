# New API Direct Deployment

Docker Compose deployment for New API with Postgres and Redis.

## Usage

1. Copy `.env.example` to `.env`.
2. Replace the placeholder secrets in `.env`.
3. Choose a deployment mode.

### Public image mode

```sh
docker compose --env-file .env up -d
```

This uses `calciumion/new-api:${NEW_API_VERSION:-latest}`.

### Local code build mode

```sh
./up-local.sh
```

This builds the `new-api` service from the local repository root via `Dockerfile`, so your local code changes are included in the image.

If you prefer the raw Compose command:

```sh
docker compose \
  -p new-api-direct-fixed \
  --env-file .env \
  -f docker-compose.yml \
  -f docker-compose.local-build.yml \
  up -d --build
```

### Prebuilt custom image mode

Build and push the image on another machine, then set `NEW_API_IMAGE` in `.env`:

```sh
NEW_API_IMAGE=ghcr.io/your-org/new-api:20260424
```

Deploy on the server with:

```sh
./up-image.sh
```

This pulls your prebuilt `new-api` image and starts the same Postgres, Redis, volumes, ports, and environment settings from `docker-compose.yml`. The server does not build the application image.

## Local workflow

When you sync upstream open-source changes into your local repo and keep your own custom patches on top, rebuild with:

```sh
./up-local.sh
```

The New API service listens on `127.0.0.1:3001` by default.

## Notes

- In local code build mode, only the `new-api` service is built locally. `postgres` and `redis` still use their official images.
- In prebuilt custom image mode, only `new-api` is pulled from `NEW_API_IMAGE`. `postgres` and `redis` still use their official images.
- `data/` and `logs/` are runtime directories and are intentionally not tracked.
- Database and Redis credentials are loaded from `.env`.
