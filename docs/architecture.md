# Yevqo architecture

## Single-owner Control Plane

Yevqo uses one central Control Plane operated by the project owner. The Control Plane is installed and runs **only on the owner's Android device in Termux**. It is not a service that every Yevqo user installs, and users do not receive their own Control Plane instance.

The owner's Android device hosts the central application components:

- Control Plane web application and API
- Authentication and account/workspace coordination
- Central database and deployment orchestration
- Agent WebSocket endpoint

The owner exposes the panel to users through the configured public endpoint (for example, Cloudflare Tunnel). Users open the panel in an ordinary web browser; they do not need to install the Control Plane.

## User-owned devices and workloads

Each user connects their own Android/Termux device by installing and configuring the standalone Yevqo Agent package. The Agent establishes an outbound authenticated connection to the owner's Control Plane using that user's Agent pairing token.

- Users' Agents run on users' own devices.
- Builds, application processes, site files and runtime workloads are executed/stored on the connected user's device, subject to the capabilities of that Agent.
- Yevqo does not provide the users' hosting compute, disk space or Android devices.
- The owner-operated Control Plane coordinates requests and displays the state reported by Agents; it is not intended to host every user's application workload itself.

The `yevqo-all` package is for the owner's device when it needs both Control Plane and Agent locally. The standalone `yevqo` package is for a user's device that only needs an Agent to connect to the central panel.

## Availability and trust boundaries

The central panel and its API depend on the owner's Android device and its network connection being online. If that device or the Control Plane is offline, users cannot access the panel or submit new orchestration requests. Applications already running on users' devices may continue running independently, depending on their local process supervision, network and power state.

This is a multi-user service with a single central Control Plane. Account, workspace, project, device, deployment, log and secret access must be authorized and scoped to the owning tenant on the server side. A connected Agent must be bound to its authenticated user and must not be able to access another user's resources. Do not treat UI filtering as an authorization boundary.

## Explicit non-goals

- Installing a separate Control Plane for every user.
- Running user workloads on the owner's Android device by default.
- Requiring users to rent or configure a VPS for the Yevqo Control Plane.
- Sharing the owner's device resources as free hosting capacity for user workloads.

This document describes the intended deployment and trust model. Individual features and enforcement controls should be considered implemented only when verified in the corresponding server and Agent code.