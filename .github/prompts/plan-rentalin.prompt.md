## Plan: Technical Design Rentalin Backend

Menyusun Technical Design Document end-to-end untuk platform rental berbasis model produk umum (bukan khusus PlayStation), dengan arsitektur maintainable, efisien di VM 1GB RAM, siap CI/CD, dan siap evolusi ke multi-tenancy dengan isolasi data ketat.

**Scope Baseline (Decision Alignment)**
- Backend tetap menggunakan Fiber (sesuai kondisi repo sekarang) agar minim risiko migrasi dan footprint ringan.
- Multi-tenancy disiapkan sejak awal: tenant_id pada tabel inti + kebijakan RLS di Supabase.
- Kerusakan saat retur dicatat terpisah dari late fee agar pelaporan dan penagihan lebih akurat.

**Steps**
1. Phase A - Requirement Refinement & Domain Rules
1.1 Definisikan workflow inti: cek ketersediaan, pembuatan transaksi rental, serah-terima, retur, hitung denda, tutup transaksi.  
1.2 Tetapkan status machine untuk item dan transaksi agar tidak ambigu.  
1.3 Tetapkan aturan denda: late fee berbasis keterlambatan waktu, damage fee berbasis asesmen kerusakan, keduanya terpisah namun teragregasi pada settlement.  
1.4 Definisikan proses perpanjangan manual: validasi konflik jadwal, update due date, rekalkulasi biaya, audit trail.  
1.5 Definisikan edge cases dan fallback operasional (barang hilang, retur sebagian, overbooking race condition, transaksi dibatalkan).

2. Phase B - System Architecture & Infrastructure Design
2.1 Terapkan pola Clean/Hexagonal ringan untuk Go: entrypoint, transport, use case, domain, repository, infra adapter.  
2.2 Rumuskan boundary package agar dependensi satu arah (transport -> usecase -> domain, infra melalui interface).  
2.3 Definisikan strategi memory-safe pada VM Azure Standard_B2ats_v2 (1GB): batasi goroutine liar, pooling koneksi DB konservatif, pagination wajib, timeout ketat, backpressure request.  
2.4 Definisikan container strategy: multi-stage build, static binary, image minimal, non-root user, healthcheck, limit resource.  
2.5 Definisikan pipeline GitHub Actions: lint/test, build image, scan, push registry, deploy ke Azure VM dengan rolling restart sederhana.  
2.6 Definisikan observability minimal: structured log, request id, basic metrics endpoint, error budget indicator.

3. Phase C - Data Modeling (General Product Abstraction + Tenant Ready)
3.1 Finalisasi ERD berbasis products dan categories sebagai inti, bukan tabel khusus playstation.  
3.2 Definisikan tabel master dan transaksi: tenants, users, memberships, categories, products, product_items, rentals, rental_items, fee_lines, extensions, condition_reports, stock_movements, audit_logs.  
3.3 Tetapkan normal form dan constraint: unique per tenant, check constraint status enum, foreign key berjenjang, soft-delete strategy terbatas pada master.  
3.4 Tetapkan indexing strategy awal: tenant_id + kolom pencarian utama, index jadwal untuk availability lookup, index status aktif.  
3.5 Tetapkan strategi JSONB untuk atribut produk spesifik kategori (schema ringan + validasi aplikasi + optional JSON schema check).

4. Phase D - API Contract & Security
4.1 Definisikan REST endpoint per bounded context: auth/session, tenants, categories, products, inventory items, rentals, returns, fees, reports.  
4.2 Definisikan kontrak request/response termasuk error envelope baku, pagination, filtering, sorting, idempotency key untuk aksi kritikal.  
4.3 Definisikan middleware autentikasi Supabase JWT: verifikasi signature JWKS, validasi iss/aud/exp, cache key set, leeway clock skew.  
4.4 Definisikan middleware tenant context: ambil tenant_id dari claim/membership, enforce pada setiap query repository.  
4.5 Definisikan authorization matrix berbasis role (owner/staff/viewer) per tenant.

5. Phase E - Scalability & Future-Proofing
5.1 Multi-product expansion: gunakan products + product_items + attributes_jsonb; standard fields tetap typed, atribut unik disimpan terstruktur JSONB per category template.  
5.2 Alternatif EAV dicatat sebagai opsi jika kebutuhan query lintas atribut custom meningkat tajam, dengan tradeoff kompleksitas query.  
5.3 Multi-tenancy isolation: RLS policy per tabel transaksional dan master tenant-scoped; service role hanya untuk job administratif tertentu.  
5.4 Definisikan trigger kapan scale out infra: p95 latency naik konsisten, memory >80% lama, connection saturation, queue backlog, error rate meningkat.  
5.5 Definisikan jalur evolusi: dari single VM ke Azure Container Apps; tambahkan Redis hanya saat read hot-spot dan rate query tidak tertahan oleh optimasi SQL/index.

6. Phase F - Verification & Delivery Checklist
6.1 Uji unit domain rule (late fee, extension, availability conflict).  
6.2 Uji integrasi repository + RLS behavior (tenant A tidak bisa akses data tenant B).  
6.3 Uji API contract (happy path + edge cases: damaged return, partial return, manual extension).  
6.4 Uji beban ringan sesuai VM target (koneksi DB, memory profile, response time).  
6.5 Uji pipeline CI/CD dari commit sampai deploy dengan rollback procedure terdokumentasi.

**Architecture Blueprint (Planned Structure)**
- cmd/api: bootstrap config, server lifecycle, graceful shutdown.
- internal/domain: entity, value object, domain rule.
- internal/usecase: orchestration business flow (rental create/extend/return).
- internal/port: interface repository/service boundary.
- internal/adapter/http: handler, middleware, dto mapper.
- internal/adapter/postgres: implementasi repository berbasis SQL/GORM.
- internal/platform: config, logger, auth verifier, clock, id generator.

**Proposed ERD (Core Tables)**
- tenants: id, name, status, created_at.
- users: id, supabase_user_id, email, full_name, created_at.
- tenant_memberships: tenant_id, user_id, role, is_active.
- categories: id, tenant_id, code, name.
- products: id, tenant_id, category_id, sku, name, rental_unit, base_price, attributes_jsonb, is_active.
- product_items: id, tenant_id, product_id, serial_number, condition_status, availability_status, acquisition_date.
- rentals: id, tenant_id, customer_name, start_at, due_at, returned_at, status, subtotal, total_fees, grand_total.
- rental_items: id, tenant_id, rental_id, product_item_id, daily_rate, planned_days, actual_days, line_total.
- fee_lines: id, tenant_id, rental_id, fee_type (late|damage|other), amount, notes.
- extensions: id, tenant_id, rental_id, previous_due_at, new_due_at, reason, approved_by.
- condition_reports: id, tenant_id, rental_item_id, check_type (checkout|return), condition_grade, notes, damage_cost.
- audit_logs: id, tenant_id, actor_user_id, action, entity, entity_id, payload_jsonb, created_at.

**API Surface (Primary Endpoints)**
- Auth/session: POST /v1/auth/sync, GET /v1/auth/me.
- Tenant: GET /v1/tenants/current, PATCH /v1/tenants/current.
- Category: GET/POST/PATCH/DELETE /v1/categories.
- Product: GET/POST/PATCH/DELETE /v1/products.
- Product item: GET/POST/PATCH /v1/product-items.
- Availability: GET /v1/availability.
- Rental: POST /v1/rentals, GET /v1/rentals, GET /v1/rentals/{id}.
- Extension manual: POST /v1/rentals/{id}/extensions.
- Return: POST /v1/rentals/{id}/returns.
- Fees: POST /v1/rentals/{id}/fees (manual adjustment dengan audit).
- Reporting: GET /v1/reports/utilization, GET /v1/reports/revenue.

**Infra & Runtime Constraints (Azure 1GB RAM)**
- Gunakan image minimal (multi-stage, static binary, distroless/scratch/alpine final sesuai kebutuhan CA cert).
- Set GOMEMLIMIT dan GOGC konservatif agar GC lebih prediktif.
- Batasi DB pool (contoh awal: max open 5-10, idle 2, conn max lifetime).
- Terapkan timeout global: read, write, idle, request context.
- Hindari preload besar; query wajib selektif field + pagination.
- Gunakan graceful shutdown untuk drain koneksi saat deploy.

**Pricing Policy (MVP -> Multi-Product Ready)**
- Model: Tiered Flat Pricing (Hybrid).
- Prinsip: harga tidak di-hardcode di tabel products.
- Tambahkan tabel `price_rules` yang bisa di-scope ke `product_id` atau `category_id`.
- Contoh tier: 1-2 hari Rp50.000/hari, 3-7 hari Rp45.000/hari.
- Priority resolution: rule product-level mengalahkan category-level; jika tidak ada rule aktif, fallback ke `products.base_price`.
- Fungsi domain Go: `CalculateRentalPrice(durationDays, productID, tenantID)` agar deterministik dan mudah diunit-test.
- Benefit bisnis: insentif sewa lebih lama tanpa kompleksitas dynamic pricing.

**Schema Migration Standard**
- Wajib gunakan SQL migration tool berbasis pure SQL (rekomendasi: `golang-migrate/migrate` atau `pressly/goose`).
- Larangan di production/staging: GORM `AutoMigrate`.
- Alasan:
	- Version control skema jelas di Git.
	- Rollback presisi pada deployment GitHub Actions.
	- Kontrol penuh DDL untuk perubahan kolom/tipe/index tanpa side effect tersembunyi.
- Konvensi file migration:
	- `migrations/<timestamp>_<name>.up.sql`
	- `migrations/<timestamp>_<name>.down.sql`
- CI gate: migration harus lolos `up` dan `down` pada DB uji sebelum image dipublish.

**Observability Strategy (Lean for 1GB RAM)**
- Gunakan structured JSON logging (Zap atau Zerolog).
- Ekspos metrics Prometheus ringan via endpoint `/metrics`.
- Fokus metric awal:
	- HTTP request count/latency/error rate.
	- Go runtime memory (`go_memstats_*`) dan goroutine count.
	- DB pool stats (open, in-use, idle, wait count).
- Tetapkan OpenTelemetry sebagai fase berikutnya (saat beban/arsitektur butuh tracing terdistribusi).
- Alasan: minim overhead RAM/CPU, namun cukup untuk mendeteksi mayoritas issue produksi awal.

**CI/CD Plan (GitHub Actions -> Azure VM)**
- Trigger: push ke main + pull request.
- Job 1 (quality): go mod verify, go test, lint, race test (opsional pada PR utama).
- Job 1b (migration check): jalankan SQL migration `up` dan `down` terhadap PostgreSQL test instance.
- Job 2 (build): docker buildx multi-stage, tag commit sha + latest.
- Job 3 (security): scan image dependency.
- Job 4 (publish): push image ke GHCR/ACR.
- Job 5 (deploy): SSH/agent ke VM, pull image baru, restart container dengan healthcheck gate.
- Job 6 (post deploy): smoke test endpoint health + rollback otomatis jika gagal.

**Verification**
1. Validasi konsistensi rule bisnis via test table-driven untuk skenario rental/return/extension.
2. Validasi isolasi tenant via integration test yang meniru user lintas tenant.
3. Validasi JWT middleware dengan token valid, expired, wrong audience, wrong issuer.
4. Validasi performa baseline di environment mirip B2ats_v2 (target p95 endpoint list utama).
5. Validasi migrasi schema aman (up/down) dan tidak merusak data tenant.

**Included Scope**
- Internal management rental tanpa payment gateway.
- Pengelolaan denda keterlambatan dan kerusakan terpisah.
- Fondasi multi-product dan multi-tenant sejak awal.

**Excluded Scope**
- Integrasi pembayaran eksternal.
- Integrasi IoT/perangkat otomatis.
- Event-driven microservices penuh pada fase awal.

**Further Considerations**
1. Pricing rule governance: tetapkan siapa yang boleh mengubah `price_rules` dan kapan rule efektif berlaku (`effective_from`, `effective_until`).
2. Migration governance: tetapkan kebijakan 1 migration per perubahan skema dan review SQL wajib pada PR.
3. Observability evolution: definisikan trigger adopsi OpenTelemetry saat masuk Azure Container Apps/multi-service.
