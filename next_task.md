## Next Task

Align the product with the updated UX expectations:
- Backend: add settings persistence for base domain, GitHub token, Cloudflare token, and cloudflared config path (default to ~/.cloudflared/config.yml); use settings in workflows.
- Frontend: build a Settings view for those values with a live cloudflared config preview.
- Add host monitoring: list running Docker services/containers and allow forwarding via subdomain on the single tunnel.
- Then run `docker compose up --build` and validate template, existing, quick service, and container-forward flows end-to-end.
