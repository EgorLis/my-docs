# My Docs

![Go](https://img.shields.io/badge/Go-1.25.0-blue)  
![Postgres](https://img.shields.io/badge/Postgres-16-blue)  
![Redis](https://img.shields.io/badge/Redis-ready-red)  
![MinIO](https://img.shields.io/badge/S3-MinIO-orange)  
![Docker](https://img.shields.io/badge/Docker-ready-blue)

**My Docs** — учебный проект для хранения и управления документами с использованием Go, PostgreSQL, Redis (для кэша и black-list токенов) и MinIO (хранение файлов).  
Проект построен с учётом лучших практик: миграции БД, авторизация через JWT, работа с ACL и кешем, документация через Swagger.

---

## 🚀 Стек технологий

- **Язык:** Go 1.25.0  
- **База данных:** PostgreSQL  
  - [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool) — пул соединений  
  - [golang-migrate](https://github.com/golang-migrate/migrate) — миграции  
- **Хранилище файлов:** MinIO (S3 совместимое API)  
- **Кэш и Blacklist токенов:** Redis  
- **Инфраструктура:** Docker, docker-compose  
- **Веб-сервер:** стандартный `net/http`  
- **Логгирование:** встроенный логгер Go  
- **Taskfile:** автоматизация рутинных задач  
- **Документация:** Swagger (через swaggo/swag)

---

## 📂 Структура проекта

```bash
my-docs/
├── cmd/my-docs/       # Точка входа (main.go)
├── configs/           # Конфиги (.env, .env.docker)
├── deployments/docker # Dockerfile, docker-compose.yml
├── internal/          # Внутренняя логика
│   ├── app/           # Builder приложения
│   ├── config/        # Конфиги и ENV
│   ├── docs/          # Swagger (генерируется)
│   ├── domain/        # Доменные сущности и интерфейсы
│   ├── infra/         # Репозитории (Postgres, Redis, S3)
│   └── transport/     # HTTP API (handlers, middleware, v1)
├── Taskfile.yml       # Автоматизация задач
├── README.md          # Документация
└── go.mod / go.sum    # Зависимости
```

---

## ⚙️ Запуск проекта

### 1. Подготовка окружения

```bash
task env
```

Скопирует `.env.docker` и `.env` в проект.

### 2. Сборка и запуск приложения

```bash
task up          # запуск в форграунде
task up:detached # запуск в фоне
```

После запуска сервер доступен на:  
📍 `http://localhost:8001`

MinIO доступен на:  
📍 API: `http://localhost:9000`  
📍 WebUI: `http://localhost:9001`

Redis доступен на `localhost:6379`.

### 3. Swagger-документация

```bash
task swagger
```

Открыть:  
📍 `http://localhost:8001/swagger/index.html`

### 4. Управление контейнерами

```bash
task logs        # логи
task ps          # статус
task stop        # остановить контейнеры
task down        # удалить контейнеры (volume сохраняется)
task down:volumes # снести контейнеры и volumes
```

---

## 📡 REST API

Все ответы обёрнуты в конверт:

```jsonc
{
  "error": { "code": 401, "text": "unauthorized" },
  "response": { ... },
  "data": { ... }
}
```

### Основные ручки

#### 🔑 Аутентификация

- `POST /api/register` — регистрация нового пользователя (только админ-токен)  
- `POST /api/auth` — вход, выдача JWT  
- `DELETE /api/auth/<token>` — logout (blacklist через Redis)

#### 📄 Документы

- `POST /api/docs` — загрузка документа (meta + json + файл)  
- `GET /api/docs` — список документов (свои / публичные / доступные по ACL)  
- `GET /api/docs/{id}` — получить документ (JSON или файл)  
- `DELETE /api/docs/{id}` — удалить документ  

#### 🔒 ACL

- Документы можно делиться через `doc_shares` (grant на чтение).  
- Владелец управляет доступами.

---

## 📖 Примеры cURL-запросов

### 1. Регистрация пользователя (админ-токен)

```bash
curl -X POST http://localhost:8001/api/register   -H "Content-Type: application/json"   -d '{"token":"ADMIN_TOKEN","login":"testuser1","pswd":"Qwerty123!"}'
```

### 2. Авторизация

```bash
curl -X POST http://localhost:8001/api/auth   -H "Content-Type: application/json"   -d '{"login":"testuser1","pswd":"Qwerty123!"}'
```

Ответ:

```json
{
  "response": {
    "token": "JWT_TOKEN"
  }
}
```

### 3. Загрузка документа

```bash
curl -X POST http://localhost:8001/api/docs   -H "Authorization: Bearer JWT_TOKEN"   -F 'meta={"name":"note.json","file":false,"public":false}'   -F 'json={"hello":"world"}'
```

### 4. Получение списка документов

```bash
curl -X GET http://localhost:8001/api/docs   -H "Authorization: Bearer JWT_TOKEN"
```

### 5. Получение документа по ID

```bash
curl -X GET http://localhost:8001/api/docs/DOC_UUID   -H "Authorization: Bearer JWT_TOKEN"
```

### 6. Удаление документа

```bash
curl -X DELETE http://localhost:8001/api/docs/DOC_UUID   -H "Authorization: Bearer JWT_TOKEN"
```

### 7. Logout (ревокация токена)

```bash
curl -X DELETE http://localhost:8001/api/auth/JWT_TOKEN
```

---

## 📖 Полезные команды

```bash
task help        # список задач
task clean       # очистка dangling образов
task swagger     # генерация Swagger доков
```

---

## 📌 Репозиторий

[🔗 GitHub: EgorLis/my-docs](https://github.com/EgorLis/my-docs)
