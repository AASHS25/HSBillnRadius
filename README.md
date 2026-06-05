# Billing + RADIUS ISP (Golang)

Platform **Billing & RADIUS** multi-tenant untuk ISP / RTRWNet / Mini ISP:
AAA (PPPoE/Hotspot/DHCP via RADIUS), billing otomatis, auto-isolir/restore via
CoA, notifikasi WhatsApp, payment gateway, voucher, dan manajemen jaringan
(OLT/GenieACS).

Repo ini adalah **modular monolith** (satu Go module) yang dipecah menjadi
beberapa binary supaya mudah di-scale dan tahan kegagalan — jika web/API tumbang,
RADIUS tetap melayani auth pelanggan.

> Status: **M0 — Bootstrap** selesai. Lihat [Roadmap](#roadmap).

## Arsitektur singkat

| Binary | Tanggung jawab |
|---|---|
| `cmd/api` | REST API + WebSocket (admin, operator, reseller, client area, webhook) |
| `cmd/radius` | Server RADIUS (Auth + Accounting) + pengirim CoA/Disconnect |
| `cmd/worker` | Konsumen queue: notifikasi WA/email, retry webhook, polling OLT/ACS |
| `cmd/scheduler` | Cron: generate invoice, reminder, scan isolir, housekeeping |
| `cmd/migrate` | Menjalankan migrasi DB (goose, embedded) |

Infra bersama: **PostgreSQL 16** (data domain + tabel RADIUS) dan **Redis**
(cache, rate-limit, queue, pub/sub, lock).

### Keputusan kunci

- **PostgreSQL** (bukan MySQL): partisi tabel `radacct`, JSONB config tenant,
  performa concurrent write.
- **sqlc + pgx/v5**: query type-safe, tanpa ORM berat di jalur panas.
- **river** (queue Postgres-native, transactional) untuk job async.
- **layeh.com/radius** untuk encode/decode paket RADIUS (termasuk Mikrotik VSA).
- **Pisah binary** sejak awal (isolasi kegagalan, scale horizontal).
- Clean architecture: `transport → service → domain`, `service → ports (interface)`;
  `domain` tidak meng-import apa pun dari luar.

## Struktur direktori

```
cmd/                 # entrypoint tiap binary
internal/
  config/            # load & validate env -> Config (fail-fast)
  platform/          # adapter infra: postgres, redis, logger, httpserver
  domain/            # entitas + business rules (tanpa dependensi luar)  [mulai M1+]
  service/           # use-cases (orkestrasi domain + ports)             [mulai M1+]
  ports/             # interface (repo, gateway) + adapter outbound      [mulai M1+]
  transport/         # delivery: httpapi, radiusapi                      [mulai M1+]
  repo/              # sqlc-generated + implementasi repository
migrations/          # goose .sql (di-embed ke cmd/migrate)
deploy/              # docker-compose + Dockerfile
configs/             # contoh .env, dictionary RADIUS
```

## Prasyarat

- Go **1.25+** (diwajibkan oleh `goose`; toolchain auto-download jika perlu)
- Docker + Docker Compose (untuk Postgres & Redis lokal)
- `golangci-lint` v2.x (untuk lint)
- Opsional: `goose`, `sqlc` (`make tools` untuk install)

## Mulai cepat (development)

```bash
# 1. Siapkan env
cp configs/.env.example .env

# 2. Jalankan infra (Postgres + Redis)
make infra-up

# 3. Jalankan migrasi DB
make migrate-up

# 4. Jalankan api-service
make run-api
# -> http://localhost:8080/healthz  dan  /readyz  dan  /metrics
```

Cek kesehatan:

```bash
curl -s localhost:8080/healthz   # {"status":"ok"}
curl -s localhost:8080/readyz    # cek Postgres + Redis
```

## Perintah Make

```bash
make help          # daftar semua target
make build         # build semua binary ke bin/
make test          # go test -race -cover
make lint          # golangci-lint
make vet           # go vet
make ci            # build + vet + test + lint
make infra-up      # start Postgres + Redis
make infra-down    # stop infra
make migrate-up    # apply migrasi
make migrate-status
```

## Konfigurasi

Semua via environment (12-factor), divalidasi saat boot. Lihat
[`configs/.env.example`](configs/.env.example) untuk daftar lengkap variabel
(`APP_*`, `HTTP_*`, `POSTGRES_*`, `REDIS_*`, `RADIUS_*`).

## API (M1)

Semua di bawah `/api/v1`. Auth pakai `Authorization: Bearer <access_token>`.

| Method | Path | Auth | Keterangan |
|---|---|---|---|
| POST | `/auth/register` | - | Buat tenant baru + user owner, balas sesi |
| POST | `/auth/login` | - | `{tenant_slug,email,password}` → sesi |
| POST | `/auth/refresh` | - | Rotasi refresh token (deteksi reuse) |
| POST | `/auth/logout` | - | Revoke refresh token |
| GET | `/me` | Bearer | Profil user + tenant + permissions |
| GET | `/users` | `user.read` | Daftar user (tenant-scoped, paginated) |
| GET | `/audit-logs` | `tenant.read` | Audit log tenant |
| CRUD | `/plans` | `plan.manage` | Paket + bandwidth profile → sync radgroupreply |
| CRUD | `/customers` | `customer.*` | Pelanggan → sync radcheck/radusergroup (PPPoE) |
| POST | `/invoices/generate` | `invoice.manage` | Generate invoice bulanan (prorata) |
| GET | `/invoices`, `/invoices/{id}` | `invoice.read` | Daftar / detail invoice + items |
| POST | `/invoices/{id}/pay` | `payment.manage` | Bayar → ledger + extend + restore |
| GET | `/reports/summary` | `invoice.read` | Pemasukan/pengeluaran/tunggakan |

```bash
curl -X POST localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"tenant_name":"Acme ISP","tenant_slug":"acme","name":"Owner","email":"owner@acme.test","password":"password123"}'
```

## Roadmap

- [x] **M0 — Bootstrap**: struktur, config, logger, pgx pool, redis, Makefile,
  docker-compose, goose, golangci-lint, healthz/readyz, CI.
- [x] **M1 — Tenancy & Auth**: tenants/users/roles/permissions/refresh_tokens,
  register/login/refresh/logout, argon2id, JWT access+refresh (rotating +
  reuse detection), RBAC middleware, multi-tenant context, audit log. sqlc.
- [x] **M2 — Customer & Plan**: CRUD pelanggan, paket, bandwidth profile,
  mapping otomatis ke radcheck/radusergroup/radgroupreply (Mikrotik-Rate-Limit,
  Framed-Pool), grup isolir. sqlc + in-memory repo untuk test.
- [x] **M3 — RADIUS (Auth)**: UDP server + bounded worker pool, NAS resolution
  (cached), PAP/CHAP auth vs radcheck, Mikrotik VSA reply (rate-limit, pool),
  radpostauth, Redis cache. Verified dengan klien RADIUS nyata.
- [x] **M4 — Accounting + CoA**: radacct (partisi bulanan), async batch writer,
  Start/Interim/Stop, Accounting-Response, CoA/Disconnect client, Isolate/Restore
  (ganti grup + kick sesi). Verified Start/Interim/Stop end-to-end.
- [x] **M5 — Billing**: invoice generator + prorata (fungsi murni), pajak (bps,
  int64), status machine, pembayaran manual → ledger income + extend masa aktif +
  restore RADIUS, laporan, scheduler (mark-overdue + auto-isolir scan). Verified.
- [x] **M6 — Notifikasi WA**: interface WAClient + 2 provider (fonnte, wablas),
  antrian transaksional (notification_logs sebagai queue, dedup idempoten,
  backoff retry), template render, worker processor. Auto-notif "paid" dari
  billing. Verified end-to-end (mock provider).
- [ ] **M7 — Payment Gateway**
- [ ] **M8 — Voucher & Hotspot** · **M9 — Jaringan (OLT/ACS)** · **M10 — Client area + hardening**

## Konvensi engineering

- Uang selalu `int64` (rupiah penuh) — tidak pernah `float64`.
- Semua I/O eksternal pakai `context.Context` dengan timeout.
- Error di-wrap dengan konteks (`fmt.Errorf("...: %w", err)`); tidak ada `panic`
  di jalur request.
- Tidak ada SQL string concatenation dari input user (parameterized query).
- Structured logging (slog JSON) dengan `trace_id`/`tenant_id`.
- Setiap package punya test; target coverage domain & service ≥ 70%.
