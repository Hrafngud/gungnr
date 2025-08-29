#!/usr/bin/env zsh

# Gungnr - Galidor Panel API CLI Masterpiece
# Usage: gungnr [login | get <endpoint> [<id>] | post <endpoint> <json> | patch <endpoint> <id> <json> | delete <endpoint> <id>]
#   Alias: alias gungnr='~/scripts/gungnr.sh'
#   Token: ~/.gungnr_token (auto-loaded)
#   Requires: jq (sudo pacman -S jq)
#   Supports any endpoint path (e.g., auth/users, leads, someroute/somechildroute)

BASE_URL="http://localhost:3000/api"
TOKEN_FILE="$HOME/.gungnr_token"
[[ -f $TOKEN_FILE ]] && source $TOKEN_FILE

# Curl wrapper: returns JSON body and HTTP status
curl_cmd() {
    curl -s -w "\nHTTP_STATUS:%{http_code}" "$@"
}

# Pretty-print JSON
print_response() {
    local json_body=$1 http_status=$2
    print "Response (HTTP $http_status):"
    command -v jq >/dev/null 2>&1 && echo "$json_body" | jq . || print "$json_body"
}

# Login: authenticate and save token
login() {
    print -n "Email: "; read email
    print -n "Password: "; read -s password; print
    print -n "Keep logged in? (t/f, default f): "; read keepLoggedIn
    keepLoggedIn=${keepLoggedIn:-false}
    [[ $keepLoggedIn == [tT]* ]] && keepLoggedIn="true" || keepLoggedIn="false"
    body="{\"email\":\"$email\",\"password\":\"$password\",\"keepLoggedIn\":$keepLoggedIn}"
    print "Request body: $body"
    response=$(curl_cmd -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" -d "$body")
    http_status=${response##*HTTP_STATUS:}
    json_body=${response%$'\nHTTP_STATUS:'*}
    print_response "$json_body" $http_status
    [[ $http_status -eq 200 && $json_body == *"accessToken"* ]] || { print "Login failed (HTTP $http_status)"; return 1; }
    token=$(echo "$json_body" | grep -o '"accessToken":"[^"]*' | cut -d'"' -f4)
    refresh_token=$(echo "$json_body" | grep -o '"refreshToken":"[^"]*' | cut -d'"' -f4)
    [[ -n $token ]] || { print "Failed to extract accessToken"; return 1; }
    echo "export API_TOKEN=$token" > $TOKEN_FILE
    chmod 600 $TOKEN_FILE
    export API_TOKEN=$token
    print "Login successful. Token saved to $TOKEN_FILE"
    print "Refresh token: $refresh_token"
}

# Generic request handler
request() {
    local method=$1 endpoint=$2 body=$3 id=$4
    [[ -z $API_TOKEN ]] && { print "No API_TOKEN set. Run 'gungnr login'"; return 1; }
    url="$BASE_URL/$endpoint${id:+/$id}"
    response=$(curl_cmd -X $method "$url" -H "Authorization: Bearer $API_TOKEN" -H "Content-Type: application/json" ${body:+-d "$body"})
    http_status=${response##*HTTP_STATUS:}
    json_body=${response%$'\nHTTP_STATUS:'*}
    print_response "$json_body" $http_status
}

# Main logic
case $1 in
    login) login ;;
    get) [[ -n $2 ]] || { print "Use: get <endpoint> [<id>]"; exit 1; }; request GET $2 "" $3 ;;
    post) [[ -n $2 && -n $3 ]] || { print "Use: post <endpoint> <json_body>"; exit 1; }; request POST $2 "$3" ;;
    patch) [[ -n $2 && -n $3 && -n $4 ]] || { print "Use: patch <endpoint> <id> <json_body>"; exit 1; }; request PATCH $2 "$4" $3 ;;
    delete) [[ -n $2 && -n $3 ]] || { print "Use: delete <endpoint> <id>"; exit 1; }; request DELETE $2 "" $3 ;;
    *) print "Usage: gungnr [login | get <endpoint> [<id>] | post <endpoint> <json_body> | patch <endpoint> <id> <json_body> | delete <endpoint> <id>]"; exit 1 ;;
esac
