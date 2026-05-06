# Configuración de Azure DevOps CLI - Perfiles yFlow y ySocial

## Objetivo

Configurar el CLI `ado` para trabajar con los proyectos **yFlow** e **ySocial** de la organización `https://yoizen.visualstudio.com`.

---

## Requisitos

### 1. Crear dos perfiles

| Perfil | Proyecto | Acceso |
|--------|----------|--------|
| `yoizen-yflow` | yFlow | PRs y repositorios (sin work items) |
| `yoizen-ysocial` | ySocial | Work Items (lectura, modificación, etc.) y PRs |

### 2. Organización
Ambos perfiles usan: `https://yoizen.visualstudio.com`

### 3. PAT (Personal Access Token)
- **NO hardcodear ningún token**
- El PAT se ingresa de forma segura cuando el CLI lo solicite
- Ambos perfiles comparten el mismo PAT (se almacena por organización)

---

## Pasos de Configuración

### Paso 1: Agregar perfil yoizen-yflow

```bash
ado profile add --name yoizen-yflow --org https://yoizen.visualstudio.com --project yFlow
```

### Paso 2: Agregar perfil yoizen-ysocial

```bash
ado profile add --name yoizen-ysocial --org https://yoizen.visualstudio.com --project ySocial
```

### Paso 3: Configurar permisos (scopes) por perfil

```bash
# yoizen-yflow: solo repos y PRs (sin work items)
ado profile set-permissions --name yoizen-yflow --scopes repos,prs

# yoizen-ysocial: work items + repos + PRs
ado profile set-permissions --name yoizen-ysocial --scopes workitems,repos,prs
```

### Paso 4: Autenticación (primera vez)

```bash
ado auth login --profile yoizen-yflow
```

Cuando el CLI lo solicite, **ingresa tu PAT de forma segura**. No lo escribas en ningún archivo.

> Los perfiles de la misma organización comparten automáticamente el PAT.

### Paso 5: Verificar sincronización

```bash
ado profile sync
```

Este comando mostrará que ambos perfiles comparten la misma organización.

---

## Testing

### Verificar acceso yoizen-ysocial (Work Items)

```bash
ado auth test --profile yoizen-ysocial
```

**Esperado:** Debe mostrar acceso exitoso.

### Verificar acceso yoizen-yflow (solo PRs y repos)

```bash
ado auth test --profile yoizen-yflow
```

**Esperado:** Debe mostrar acceso exitoso.

---

## Comandos Útiles

### Listar perfiles
```bash
ado profile list
```

### Ver permisos de un perfil
```bash
ado profile show-permissions --name yoizen-yflow
ado profile show-permissions --name yoizen-ysocial
```

### Usar un perfil específico
```bash
ado work-item list --profile yoizen-ysocial --mine
ado pr list --profile yoizen-yflow --repo <nombre-repo>
```

### Cambiar perfil por defecto
```bash
ado profile use --name yoizen-ysocial
```

---

## Scopes Disponibles

| Scope | Descripción |
|-------|-------------|
| `workitems` | Acceso a comandos de work items |
| `repos` | Acceso a comandos de repositorios |
| `prs` | Acceso a comandos de pull requests |

---

## Nota de Seguridad

> **IMPORTANTE:** Nunca guardes el PAT en archivos de configuración, código o historial de comandos. Ingrésalo únicamente cuando el CLI lo solicite de forma interactiva.
