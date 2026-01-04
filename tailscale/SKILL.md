---
name: tailscale
description: Manage Tailscale VPN, Serve, and Funnel from the CLI.
homepage: https://tailscale.com/
metadata: {"clawdis":{"emoji":"🧭","requires":{"bins":["tailscale"]},"install":[{"id":"brew","kind":"brew","formula":"tailscale","bins":["tailscale"],"label":"Install Tailscale CLI (brew)"}]}}
---

# Tailscale CLI

Use `tailscale` to inspect status, fetch tailnet IPs, and expose services.

Quick start
- `tailscale status`
- `tailscale ip -4`
- `tailscale serve 3000`
- `tailscale funnel 3000`

Notes
- `serve` shares a local service inside your tailnet; `funnel` exposes it publicly.
- Serve/Funnel CLI changed in Tailscale 1.52+; use `--help` for exact flags.
