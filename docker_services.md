# Useful Docker Containers for Developers (TS/Vue/Go Workflow)

A curated collection of lightweight, self-hosted Docker services to boost your local development setup. All commands include persistent volumes where needed and `--restart unless-stopped` for reliability. Adjust ports/volumes as necessary.

## Diagramming and Prototyping
- **Excalidraw** (collaborative whiteboard for diagrams/architecture sketching)
  ```bash
  docker run -d -p 80:80 --name excalidraw --restart unless-stopped excalidraw/excalidraw:latest
  ```
  Access: `http://localhost`

## AI Assistance
- **OpenWebUI** (self-hosted UI for local LLMs like Ollama)
  ```bash
  docker run -d -p 3000:8080 -v open-webui:/app/backend/data --name open-webui --restart unless-stopped ghcr.io/open-webui/open-webui:main
  ```
  Access: `http://localhost:3000`

- **Ollama** (run local LLMs - pair with OpenWebUI)
  ```bash
  docker run -d -p 11434:11434 --name ollama -v ollamadata:/root/.ollama --restart unless-stopped ollama/ollama:latest
  ```
  Pull models: `docker exec ollama ollama pull llama3`

## Backends and Databases
- **PocketBase** (lightweight Go-based backend with realtime DB, auth, admin UI)
  ```bash
  docker run -d -p 8090:8090 --name pocketbase -v $(pwd)/pb_data:/pb_data --restart unless-stopped ghcr.io/muchobien/pocketbase:latest
  ```
  Admin: `http://localhost:8090/_`

- **PostgreSQL**
  ```bash
  docker run -d -p 5432:5432 --name postgres -e POSTGRES_PASSWORD=yourpassword -v pgdata:/var/lib/postgresql/data --restart unless-stopped postgres:latest
  ```

- **MySQL**
  ```bash
  docker run -d -p 3306:3306 --name mysql -e MYSQL_ROOT_PASSWORD=yourpassword -v mysqldata:/var/lib/mysql --restart unless-stopped mysql:latest
  ```

- **Redis** (cache/queue)
  ```bash
  docker run -d -p 6379:6379 --name redis -v redisdata:/data --restart unless-stopped redis:latest
  ```

- **MongoDB** (NoSQL)
  ```bash
  docker run -d -p 27017:27017 --name mongodb -v mongodata:/data/db --restart unless-stopped mongo:latest
  ```

- **MeiliSearch** (fast search engine)
  ```bash
  docker run -d -p 7700:7700 --name meilisearch -v meilidata:/meili_data --restart unless-stopped getmeili/meilisearch:latest
  ```

## Web Servers and Proxies
- **Nginx** (static serving / reverse proxy)
  ```bash
  docker run -d -p 80:80 --name nginx -v $(pwd)/html:/usr/share/nginx/html --restart unless-stopped nginx:latest
  ```

- **Traefik** (reverse proxy with auto-SSL and Docker integration)
  ```bash
  docker run -d -p 80:80 -p 443:443 -p 8080:8080 --name traefik -v /var/run/docker.sock:/var/run/docker.sock -v traefikdata:/data --restart unless-stopped traefik:latest --api.insecure=true --providers.docker
  ```
  Dashboard: `http://localhost:8080`

## Storage and Files
- **MinIO** (S3-compatible object storage)
  ```bash
  docker run -d -p 9000:9000 -p 9001:9001 --name minio -v miniodata:/data -e "MINIO_ROOT_USER=admin" -e "MINIO_ROOT_PASSWORD=password123" --restart unless-stopped quay.io/minio/minio server /data --console-address ":9001"
  ```
  Console: `http://localhost:9001` (admin / password123)

## Testing and Debugging
- **MailHog** (email capture for testing)
  ```bash
  docker run -d -p 1025:1025 -p 8025:8025 --name mailhog --restart unless-stopped mailhog/mailhog:latest
  ```
  UI: `http://localhost:8025`

- **Hoppscotch** (open-source API client like Postman)
  ```bash
  docker run -d -p 3000:3000 --name hoppscotch --restart unless-stopped hoppscotch/hoppscotch:latest
  ```
  Access: `http://localhost:3000`

- **IT-Tools** (JSON formatter, encoders, regex tester, etc.)
  ```bash
  docker run -d -p 8080:80 --name it-tools --restart unless-stopped corentinth/it-tools:latest
  ```
  Access: `http://localhost:8080`

## Admin and Management
- **Adminer** (lightweight DB admin)
  ```bash
  docker run -d -p 8080:8080 --name adminer --restart unless-stopped adminer:latest
  ```
  Access: `http://localhost:8080`

- **Mongo Express** (MongoDB web admin)
  ```bash
  docker run -d -p 8081:8081 --name mongo-express -e ME_CONFIG_MONGODB_SERVER=mongodb -e ME_CONFIG_BASICAUTH_USERNAME=admin -e ME_CONFIG_BASICAUTH_PASSWORD=pass --restart unless-stopped mongo-express:latest
  ```
  Access: `http://localhost:8081`

- **Portainer** (Docker management UI)
  ```bash
  docker run -d -p 9000:9000 -p 9443:9443 --name portainer -v /var/run/docker.sock:/var/run/docker.sock -v portainerdata:/data --restart unless-stopped portainer/portainer-ce:latest
  ```
  Access: `http://localhost:9000`

- **Watchtower** (auto-update containers)
  ```bash
  docker run -d --name watchtower -v /var/run/docker.sock:/var/run/docker.sock containrrr/watchtower --cleanup --interval 300
  ```

## Development Environments
- **Code-Server** (VS Code in the browser)
  ```bash
  docker run -d -p 8080:8080 --name code-server -v $(pwd):/home/coder/project -e PASSWORD=yourpassword --restart unless-stopped codercom/code-server:latest
  ```
  Access: `http://localhost:8080`

## Version Control
- **Gitea** (lightweight self-hosted Git)
  ```bash
  docker run -d -p 3000:3000 -p 2222:22 --name gitea -v giteadata:/data --restart unless-stopped gitea/gitea:latest
  ```
  Access: `http://localhost:3000`

These services cover prototyping, databases, caching, testing, monitoring, and remote dev - perfect for full-stack TS/Vue/Go projects. Many play nicely together in a `docker-compose.yml` for quick stacks.
