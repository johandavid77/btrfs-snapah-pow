# BTRFS Snapah Pow

<p align="center">
  <img src="assets/logo.svg" alt="Snapah Pow" width="600"/>
</p>



![Snapah Pow](assets/logo.svg)
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