[![CI](https://github.com/johandavid77/btrfs-snapah-pow/actions/workflows/ci.yml/badge.svg)](https://github.com/johandavid77/btrfs-snapah-pow/actions/workflows/ci.yml)
[![Release](https://github.com/johandavid77/btrfs-snapah-pow/actions/workflows/release.yml/badge.svg)](https://github.com/johandavid77/btrfs-snapah-pow/actions/workflows/release.yml)
[![Docker](https://ghcr-badge.egpl.dev/johandavid77/btrfs-snapah-pow/size)](https://github.com/johandavid77/btrfs-snapah-pow/pkgs/container/btrfs-snapah-pow)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

# BTRFS Snapah Pow

<p align="center">
  <img src="assets/logo.svg" alt="Snapah Pow" width="600"/>
</p>



Gestion centralizada de snapshots BTRFS para entornos multi-nodo.
Servidor central + agentes distribuidos + CLI comunicados por gRPC.

## Arquitectura



## Instalacion rapida



## Uso



## REST API

| Metodo | Endpoint              | Descripcion               |
|--------|-----------------------|---------------------------|
| GET    | /health               | Estado del servidor       |
| GET    | /api/nodes            | Listar nodos              |
| GET    | /api/snapshots        | Listar snapshots          |
| POST   | /api/snapshots        | Crear snapshot            |
| POST   | /api/snapshots/delete | Eliminar snapshot         |
| GET    | /api/events           | Eventos recientes         |
| GET    | /api/policies         | Listar politicas cron     |
| POST   | /api/policies         | Crear politica programada |

## Crear snapshot via API



## Variables de entorno

| Variable      | Descripcion                 |
|---------------|-----------------------------|
| SNAPAH_SERVER | Direccion del servidor gRPC |
| SNAPAH_TOKEN  | Token de autenticacion      |
| SNAPAH_CONFIG | Ruta al config.yaml         |

## Estructura



## Licencia

GPL3 2026 Johan David
---

## Roadmap

### v0.1.0 — Base funcional (actual)
- [x] Arquitectura servidor + agente + CLI
- [x] gRPC con Protocol Buffers
- [x] Crear, listar y eliminar snapshots BTRFS
- [x] Registro y heartbeat de nodos
- [x] SQLite embebido con GORM
- [x] Scheduler con expresiones cron reales
- [x] Retención automática de snapshots
- [x] REST API completa
- [x] Streaming de eventos gRPC
- [x] Instalación como servicio systemd
- [x] IDs con UUID (sin colisiones)

### v0.2.0 — Seguridad y autenticación
- [x] JWT en endpoints HTTP
- [x] mTLS entre servidor y agentes
- [x] Validación real de tokens de registro
- [x] RBAC (roles: admin, operator, viewer)
- [x] Rate limiting en API

### v0.3.0 — Web UI
- [x] Dashboard con lista de nodos en tiempo real
- [x] Tabla de snapshots con filtros
- [x] Crear y eliminar snapshots desde el navegador
- [x] Gestión de políticas cron via UI
- [x] Log de eventos en tiempo real (WebSocket)
- [x] Indicador de estado de nodos online/offline

### v0.4.0 — Replicación
- [x] btrfs send/receive entre nodos
- [x] Políticas de replicación configurables
- [x] Replicación incremental (solo delta)
- [x] Estado y progreso de replicación en tiempo real
- [x] Retry automático en fallo de red

### v0.5.0 — Observabilidad
- [x] Métricas Prometheus (/metrics)
- [x] Dashboard Grafana preconfigurado
- [x] Alertas configurables (snapshot fallido, nodo offline)
- [x] Historial de ejecuciones de políticas
- [x] Uptime monitor integrado

### v0.6.0 — Producción
- [x] Soporte PostgreSQL además de SQLite
- [x] Imagen Docker oficial
- [x] Helm chart para Kubernetes
- [x] CLI interactivo (TUI con Bubble Tea)
- [x] Documentación completa en GitHub Pages
- [x] Tests de integración end-to-end
