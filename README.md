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

## Deploy semua service (Docker)

Seluruh stack (api + radius + worker + scheduler + PostgreSQL + Redis) jalan
lewat satu perintah. Migrasi DB dijalankan otomatis oleh service `migrate`
(app service lain menunggu sampai selesai). Build **hermetik & offline** dari
`vendor/` (tidak butuh akses proxy Go saat build image).

```bash
# Build image semua binary + jalankan seluruh stack
docker compose -f deploy/docker-compose.yml --profile app up --build -d

# Cek
curl -s localhost:8080/readyz          # {"status":"ready","checks":{"postgres":"up","redis":"up"}}
docker compose -f deploy/docker-compose.yml --profile app ps
```

Port yang diekspos: `8080` (API), `1812/udp` (RADIUS auth), `1813/udp`
(accounting). CoA/Disconnect ke NAS lewat `3799/udp`.

### Onboarding pertama (siap pakai)

```bash
B=localhost:8080/api/v1
# 1. Daftar tenant + admin (self-service)
ACC=$(curl -s -X POST $B/auth/register -H 'Content-Type: application/json' \
  -d '{"tenant_name":"ISP Saya","tenant_slug":"isp","name":"Admin","email":"admin@isp.id","password":"rahasia12"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')
# 2. Daftarkan router MikroTik (NAS) + shared secret  ← WAJIB agar RADIUS jalan
curl -X POST $B/nas -H "Authorization: Bearer $ACC" -H 'Content-Type: application/json' \
  -d '{"nasname":"10.10.10.1","shortname":"mikrotik-pop1","secret":"radsecret123"}'
# 3. Buat paket PPPoE → otomatis sync radgroupreply (Mikrotik-Rate-Limit)
# 4. Buat pelanggan PPPoE → otomatis provisioning radcheck + radusergroup
```

Arahkan RADIUS client MikroTik ke `<host>:1812/1813` dengan secret yang
didaftarkan, set PPPoE pakai RADIUS — pelanggan langsung bisa auth.

### Production (server + HTTPS)

```bash
cp deploy/.env.example deploy/.env        # isi JWT_SECRET (openssl rand -base64 48), password DB, dst.
cp deploy/Caddyfile.example deploy/Caddyfile  # ganti domain
docker compose --env-file deploy/.env \
  -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml \
  --profile app up --build -d
```

Override `docker-compose.prod.yml`: `APP_ENV=production` + secret dari `.env`,
Postgres/Redis **tidak** diekspos ke host, API di belakang **Caddy** (HTTPS
auto Let's Encrypt), `restart: unless-stopped`. Buka firewall hanya untuk
`80/443/tcp` (web) dan `1812/1813/udp` (+ `3799/udp` CoA) ke radius.

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
| POST/GET | `/nas` | `tenant.update` | Daftar/registrasi router (NAS) + secret |
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
- [x] **M7 — Payment Gateway**: interface (CreateCharge/VerifyCallback), provider
  midtrans (SHA-512 signature) + xendit (callback-token), webhook publik
  `/webhooks/payment/{provider}` dengan verifikasi + idempotency + settlement
  transaksional (invoice paid + ledger + extend + restore + notif), polling
  fallback. Verified end-to-end (callback ber-signature → invoice lunas).
- [x] **M8 — Voucher & Hotspot**: batch generate + provisioning RADIUS hotspot.
- [x] **M9 — Jaringan & operasional**: tiket (state machine + timeline), peta
  pelanggan GeoJSON, **GenieACS (TR-069)** — baca/ubah Wi-Fi, reboot, refresh
  (verified vs mock NBI). Interface driver OLT multi-vendor (kontrak; impl
  per-vendor di-skip atas permintaan).
- [x] **Client area**: portal pelanggan (login terpisah, lihat invoice/tiket,
  buat tiket). **Reseller**: deposit/topup saldo + komisi otomatis saat
  pelanggan reseller bayar.
- [x] **M10 — Hardening**: security headers, rate limiting login/register (Redis
  token bucket, fail-open). Client-area portal pelanggan = sisa scope berikutnya.

## Konvensi engineering

- Uang selalu `int64` (rupiah penuh) — tidak pernah `float64`.
- Semua I/O eksternal pakai `context.Context` dengan timeout.
- Error di-wrap dengan konteks (`fmt.Errorf("...: %w", err)`); tidak ada `panic`
  di jalur request.
- Tidak ada SQL string concatenation dari input user (parameterized query).
- Structured logging (slog JSON) dengan `trace_id`/`tenant_id`.
- Setiap package punya test; target coverage domain & service ≥ 70%.
