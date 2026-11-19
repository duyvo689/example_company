# Example Company API

Backend Golang/Gin cho mô hình multi-tenant trong ERD: company, location, user, role, membership, patient, location assignment, order, audit log và integration event.

## Mô hình dữ liệu

![ERD của Example Company API](docs/images/example-company-erd.png)

## Có sẵn

- JWT access token + refresh token rotation + logout/revoke.
- RBAC với 3 role mặc định: `ADMIN`, `STAFF`, `CLINICIAN`.
- Tenant isolation theo `company_id`; `CLINICIAN` chỉ truy cập location được gán.
- CRUD đầy đủ cho company, location, custom role, membership, assignment, patient/location và order.
- Pagination, search, status filter và validation request.
- Audit mọi request làm thay đổi dữ liệu.
- Transactional outbox cho `order.created`, `order.updated`, `order.deleted`; tự gửi webhook và retry exponential backoff.
- PostgreSQL migration, graceful shutdown, health check, JSON logging, Docker/Compose và unit test.
- CORS allowlist cấu hình bằng `CORS_ALLOWED_ORIGINS` (phân tách nhiều origin bằng dấu phẩy).

## Cấu trúc

```text
cmd/api/                 entrypoint
internal/config/         cấu hình từ environment
internal/database/       kết nối, auto-migrate, seed
internal/domain/         model và request/response DTO
internal/handler/        Gin handlers
internal/middleware/     auth, active-session, RBAC, audit, logging
internal/service/        nghiệp vụ và transaction
internal/outbox/         webhook dispatcher
internal/platform/       JWT và password helpers
internal/router/         khai báo route
migrations/              PostgreSQL up/down migration
```

## Chạy nhanh bằng Docker

```bash
docker compose up --build
```

API chạy tại `http://localhost:8080`. PostgreSQL chạy tại `localhost:5432`.

Nếu cổng mặc định đang được dùng:

```bash
API_PORT=18080 POSTGRES_PORT=55432 docker compose up --build
```

Khi đổi API port, đổi `baseUrl` trong `bruno/environments/Local.bru` hoặc truyền `--env-var baseUrl=http://localhost:18080` khi chạy Bruno CLI.

Để chạy trực tiếp:

```bash
cp .env.example .env
docker compose up -d postgres
go run ./cmd/api
```

Khi dùng database đã có schema từ migration, đặt `AUTO_MIGRATE=false`. Ở môi trường local mới hoàn toàn có thể dùng `AUTO_MIGRATE=true`.

## Luồng khởi tạo

Tạo company, user và membership ADMIN đầu tiên:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "company_name": "Acme Health",
    "name": "Admin User",
    "email": "admin@acme.test",
    "password": "ChangeMe123!"
  }'
```

Response trả `access_token`, `refresh_token`, user và membership. Các API bảo vệ dùng:

```bash
curl http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Khi một email có membership ở nhiều company, truyền `company_id` trong body `/auth/login` để chọn tenant.

## API

Base URL: `/api/v1`. Danh sách endpoint chính:

| Nhóm | Endpoint |
|---|---|
| Auth | `POST /auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout` |
| Profile | `GET/PATCH /me`, `PATCH /me/password` |
| Company | `GET/PATCH /company` |
| Location | `GET/POST /locations`, `GET/PATCH/DELETE /locations/:location_id` |
| Role | `GET/POST /roles`, `PUT/DELETE /roles/:role_id` |
| Membership | `GET/POST /memberships`, `GET/PATCH/DELETE /memberships/:membership_id` |
| Assignment | `GET/POST /memberships/:membership_id/locations`, `DELETE .../locations/:location_id` |
| Patient | `GET/POST /patients`, `GET/PATCH/DELETE /patients/:patient_id` |
| Patient location | `POST /patients/:patient_id/locations`, `DELETE .../locations/:location_id` |
| Order | `GET/POST /orders`, `GET/PATCH/DELETE /orders/:order_id` |
| Audit | `GET /audit-logs`, `GET /audit-logs/:audit_log_id` |
| Outbox | `GET /integration-events`, `POST /integration-events/:event_id/retry` |
| Health | `GET /health/live`, `GET /health/ready` |

List endpoint hỗ trợ `page` và `page_size` (tối đa 100). Tùy resource còn có `search`, `status`, `location_id`, `patient_id`, `from`, `to`.

### Quyền

- `ADMIN/company`: quản trị company, location, role, membership, assignment, audit và outbox; có toàn quyền dữ liệu tenant.
- `STAFF/company`: thao tác patient/order và đọc location trong toàn company.
- `CLINICIAN/location`: chỉ đọc và thao tác patient/order thuộc location đã được gán.
- Role custom được hỗ trợ; middleware đặc quyền quản trị hiện yêu cầu code `ADMIN`. Các role mặc định không thể sửa/xóa.

Membership bị inactive, user bị inactive hoặc company bị inactive sẽ bị chặn ngay ở request kế tiếp, kể cả access token chưa hết hạn.

## Quy tắc dữ liệu

- `location.code`, `patient.public_ref`, `order.external_order_id` là duy nhất trong company.
- Khi admin thêm membership cho email chưa tồn tại, request phải kèm `name` và `password`; email đã tồn tại chỉ cần `email`, `role_id` và `status`.
- Company luôn phải còn ít nhất một membership `ADMIN` active.
- Patient phải liên kết với location trước khi tạo order ở location đó.
- Không xóa cứng location/patient/membership đang được order tham chiếu; chuyển `status=inactive` để giữ lịch sử.
- Delete ở entity chính là soft delete; bảng nối được hard delete.
- Thay đổi order và integration event được ghi chung một database transaction (outbox pattern).

## Webhook integration

Đặt `INTEGRATION_WEBHOOK_URL`. Dispatcher gửi JSON của order với headers:

- `X-Event-ID`
- `X-Event-Type`
- `Content-Type: application/json`

HTTP `2xx` được đánh dấu `published`; lỗi được retry tối đa theo lịch exponential backoff. Admin có thể đưa event về hàng đợi qua endpoint retry.

## Migration và kiểm thử

Apply thủ công:

```bash
psql "$DATABASE_URL" -f migrations/000001_init.up.sql
```

Kiểm tra code:

```bash
go test ./...
go vet ./...
```

Các secret trong `.env.example` và `docker-compose.yml` chỉ dùng local; production phải thay `JWT_SECRET`, credential database và dùng TLS/secrets manager.

## Test API bằng Bruno

Project có sẵn collection tại thư mục `bruno/`, gồm 17 request chạy tuần tự: health check, register/refresh JWT, roles, location, onboarding clinician, patient, order, audit log và integration event. Token cùng các ID được tự động lưu thành runtime variables.

### Bruno Desktop

Trên macOS, cài bằng Homebrew:

```bash
brew install --cask bruno
```

Sau đó:

1. Chạy API bằng `docker compose up --build`.
2. Mở Bruno, chọn **Open Collection**.
3. Chọn thư mục `bruno` của project.
4. Chọn environment **Local** ở góc trên bên phải.
5. Chạy từng request theo số thứ tự, hoặc chọn **Run Collection** để chạy toàn bộ.

### Bruno CLI

Bruno CLI được khai báo là dev dependency trong `package.json`. Sau khi clone project:

```bash
npm ci
npm run test:api
```

Hoặc:

```bash
make bruno-install
make test-api
```

Nếu API chạy ở cổng khác:

```bash
cd bruno
../node_modules/.bin/bru run --env Local \
  --env-var baseUrl=http://localhost:18080
```

Collection tạo email và mã tham chiếu động nên có thể chạy lại nhiều lần mà không trùng unique key. Hướng dẫn cài đặt chính thức: [Bruno Desktop](https://docs.usebruno.com/get-started/bruno-basics/download) và [Bruno CLI](https://docs.usebruno.com/bru-cli/installation).
