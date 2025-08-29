Gungnr CLI
A lightweight, powerful command-line interface (CLI) for interacting with any REST API. gungnr simplifies HTTP requests with support for flexible endpoints, pretty-printed JSON responses, and secure token management. Built for developers, it’s optimized for zsh environments like Manjaro/Linux.
Features

Flexible Endpoints: Call any API endpoint (e.g., users, posts, api/v1/resources).
Pretty JSON Output: Responses formatted with jq for readability.
HTTP Status Codes: Displays status for every request (e.g., 200, 401).
Token Persistence: Stores API token securely in ~/.gungnr_token.
Zsh-Optimized: Compact and efficient, leveraging zsh's power.

Requirements

Zsh: Runs in zsh (default on Manjaro).
jq: For pretty-printing JSON responses (sudo pacman -S jq).
REST API: A running API server (default: http://localhost:3000/api).

Installation

Clone the Repository:
git clone https://github.com/Hrafngud/gungnr.git
cd gungnr


Set Up the Script:

Move gungnr.sh to ~/scripts:mkdir -p ~/scripts
cp gungnr.sh ~/scripts/gungnr.sh
chmod +x ~/scripts/gungnr.sh


Add alias to ~/.zshrc:echo "alias gungnr='~/scripts/gungnr.sh'" >> ~/.zshrc
source ~/.zshrc




Install jq:
sudo pacman -S jq


Start Your API Server:

Ensure your API is running (e.g., at http://localhost:3000/api).
Update BASE_URL in gungnr.sh if your API uses a different host/port.



Usage
gungnr [login | get <endpoint> [<id>] | post <endpoint> <json_body> | patch <endpoint> <id> <json_body> | delete <endpoint> <id>]

Commands

Login: Authenticate and save the API token.
gungnr login


Prompts for email, password, and keepLoggedIn (enter t for true, f or empty for false).
Saves token to ~/.gungnr_token.


Get: Retrieve data from an endpoint.
gungnr get <endpoint> [<id>]


Examples:gungnr get users         # List all users
gungnr get users/1       # Get user ID 1
gungnr get api/v1/posts  # List posts from a nested endpoint




Post: Create a resource.
gungnr post <endpoint> <json_body>


Example:gungnr post users '{"name":"New User","email":"user@example.com"}'




Patch: Update a resource.
gungnr patch <endpoint> <id> <json_body>


Example:gungnr patch users/1 '{"name":"Updated User"}'




Delete: Delete a resource.
gungnr delete <endpoint> <id>


Example:gungnr delete users/1





Example Workflow

Log in:
gungnr login

Email: user@example.com
Password:
Keep logged in? (t/f, default f): t
Request body: {"email":"user@example.com","password":"your_password","keepLoggedIn":true}
Response (HTTP 200):
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
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
Login successful. Token saved to ~/.gungnr_token
Refresh token: 7f69d6ab-e636-482a-be60-d9a27176bdbc


Fetch users:
gungnr get users

Response (HTTP 200):
[
  {
    "id": "1",
    "email": "user@example.com",
    "name": "Test User",
    "role": "ADMIN",
    ...
  },
  ...
]


Create a resource:
gungnr post posts '{"title":"New Post","content":"Hello, world!"}'

Response (HTTP 201):
{
  "id": "123",
  "title": "New Post",
  "content": "Hello, world!",
  ...
}



Debugging

HTTP 401: Token expired or invalid. Run gungnr login again.
HTTP 400: Check the Request body output for invalid JSON (e.g., in login).
No JSON Formatting: Ensure jq is installed (jq --version).
Server Issues: Verify your API server is running and accessible (default: http://localhost:3000/api).

Optional: Logout Command
To clear the token, add the following to the case block in gungnr.sh:
logout) [[ -f $TOKEN_FILE ]] && { rm $TOKEN_FILE; print "Token cleared"; } || print "No token file"; unset API_TOKEN; exit 0 ;;

Update the usage message:
print "Usage: gungnr [login | logout | get <endpoint> [<id>] | post <endpoint> <json_body> | patch <endpoint> <id> <json_body> | delete <endpoint> <id>]"

Then run:
gungnr logout

Contributing
Contributions are welcome! Please:

Fork the repository.
Create a feature branch (git checkout -b feature/awesome-feature).
Commit changes (git commit -m 'Add awesome feature').
Push to the branch (git push origin feature/awesome-feature).
Open a pull request.
