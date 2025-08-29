
# Gungnr CLI

*A lightweight, powerful command-line interface (CLI) for interacting with any REST API.*

`gungnr` simplifies HTTP requests with: flexible endpoints, pretty-printed JSON responses, and secure token management.
Built for developers, it’s optimized for **zsh environments** like **Manjaro/Linux**.

---

## Features

* **Flexible Endpoints** → Call any API endpoint (`users`, `posts`, `api/v1/resources`).
* **Pretty JSON Output** → Responses formatted with `jq` for readability.
* **HTTP Status Codes** → Displays status for every request (e.g., `200`, `401`).
* **Token Persistence** → Stores API token securely in `~/.gungnr_token`.
* **Zsh-Optimized** → Compact and efficient, leveraging zsh's power.

---

## Requirements

* **Zsh** → Runs in `zsh` (default on Manjaro).
* **jq** → Pretty-print JSON (`sudo pacman -S jq`).
* **REST API** → A running API server (default: `http://localhost:3000/api`).

---

## Installation

```bash
# Clone the repository
git clone https://github.com/Hrafngud/gungnr.git
cd gungnr

# Move the script
mkdir -p ~/scripts
cp gungnr.sh ~/scripts/gungnr.sh
chmod +x ~/scripts/gungnr.sh

# Add alias to zshrc
echo "alias gungnr='~/scripts/gungnr.sh'" >> ~/.zshrc
source ~/.zshrc

# Install jq
sudo pacman -S jq
```

> Ensure your API is running (default: `http://localhost:3000/api`).
> Update `BASE_URL` in `gungnr.sh` if your API uses a different host/port.

---

## Usage

```bash
gungnr [login | get <endpoint> [<id>] | post <endpoint> <json_body> | patch <endpoint> <id> <json_body> | delete <endpoint> <id>]
```

### Commands

#### Login

Authenticate and save the API token.

```bash
gungnr login
```

* Prompts for **email**, **password**, and **keepLoggedIn** (`t` for true, `f` or empty for false).
* Saves token to `~/.gungnr_token`.

---

#### Get

Retrieve data from an endpoint.

```bash
gungnr get <endpoint> [<id>]
```

Examples:

```bash
gungnr get users          # List all users
gungnr get users/1        # Get user ID 1
gungnr get api/v1/posts   # List posts from nested endpoint
```

---

#### Post

Create a resource.

```bash
gungnr post <endpoint> <json_body>
```

Example:

```bash
gungnr post users '{"name":"New User","email":"user@example.com"}'
```

---

#### Patch

Update a resource.

```bash
gungnr patch <endpoint> <id> <json_body>
```

Example:

```bash
gungnr patch users/1 '{"name":"Updated User"}'
```

---

#### Delete

Delete a resource.

```bash
gungnr delete <endpoint> <id>
```

Example:

```bash
gungnr delete users/1
```

---

## Example Workflow

### 1. Login

```bash
gungnr login
```

Input:

```
Email: user@example.com
Password:
Keep logged in? (t/f, default f): t
```

Request body:

```json
{"email":"user@example.com","password":"your_password","keepLoggedIn":true}
```

Response (HTTP 200):

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6...",
  "refreshToken": "7f69d6ab-e636-482a-be60-d9a27176bdbc",
  "user": {
    "id": "1",
    "email": "user@example.com",
    "name": "Test User",
    "role": "ADMIN",
    "createdAt": "2025-08-28T02:56:42.907Z",
    "updatedAt": "2025-08-28T02:56:42.907Z"
  }
}
```

Token saved to `~/.gungnr_token`
Refresh token displayed

---

### 2. Fetch Users

```bash
gungnr get users
```

Response (HTTP 200):

```json
[
  {
    "id": "1",
    "email": "user@example.com",
    "name": "Test User",
    "role": "ADMIN"
  }
]
```

---

### 3. Create a Resource

```bash
gungnr post posts '{"title":"New Post","content":"Hello, world!"}'
```

Response (HTTP 201):

```json
{
  "id": "123",
  "title": "New Post",
  "content": "Hello, world!"
}
```

---

## Debugging

* **401 Unauthorized** → Token expired/invalid → `gungnr login`.
* **400 Bad Request** → Invalid JSON → Check body.
* **No JSON formatting** → Ensure `jq` installed (`jq --version`).
* **Server issues** → Verify API is running.

---

## Optional: Logout Command

Add this to `gungnr.sh` case block:

```zsh
logout) [[ -f $TOKEN_FILE ]] && { rm $TOKEN_FILE; print "Token cleared"; } || print "No token file"; unset API_TOKEN; exit 0 ;;
```

Update usage message:

```zsh
print "Usage: gungnr [login | logout | get <endpoint> [<id>] | post <endpoint> <json_body> | patch <endpoint> <id> <json_body> | delete <endpoint> <id>]"
```

Run:

```bash
gungnr logout
```
