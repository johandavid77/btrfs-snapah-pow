---
layout: default
title: snapah pow
description: Centralized BTRFS snapshot manager
---

# snapah pow

Gestor centralizado de snapshots BTRFS con servidor REST/gRPC, agentes distribuidos, CLI interactivo y Web UI.

## Instalacion rapida

```bash
# Docker
docker run -d -p 8082:8082 ghcr.io/johandavid77/btrfs-snapah-pow:latest

# Binario directo
wget https://github.com/johandavid77/btrfs-snapah-pow/releases/latest/download/snapah-server-linux-amd64
chmod +x snapah-server-linux-amd64
./snapah-server-linux-amd64
```

Credenciales por defecto: admin / admin123

## Variables de entorno

| Variable | Default | Descripcion |
|----------|---------|-------------|
| SNAPAH_ADMIN_PASSWORD | admin123 | Contrasena del admin |
| JWT_SECRET | auto | Secret para JWT |
| DATABASE_URL | data/snapah.db | SQLite o postgres://... |
| ALERT_EMAIL_TO | - | Email para alertas |
| ALERT_WEBHOOK_URL | - | Webhook Slack/Discord |

## API Reference

### Auth
POST /api/auth/login
{"username": "admin", "password": "admin123"}

### Snapshots
GET  /api/snapshots
POST /api/snapshots
POST /api/restore

### Nodos
GET /api/nodes

### Politicas
GET  /api/policies
POST /api/policies

### API Keys
GET  /api/keys
POST /api/keys
POST /api/keys/revoke

### WebSocket eventos en tiempo real
ws://localhost:8082/ws/events

## Helm / Kubernetes

```bash
helm install snapah oci://ghcr.io/johandavid77/charts/snapah-pow
kubectl port-forward svc/snapah-server 8082:8082
```
