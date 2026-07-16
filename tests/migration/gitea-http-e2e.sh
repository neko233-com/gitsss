#!/bin/sh

set -eu

source_name=gitsss-http-source
target_name=gitsss-http-target
network_name=gitsss-http-migration
source_volume=gitsss-http-source-data
target_volume=gitsss-http-target-data
source_port=33001
target_port=33002
host=${GITSSS_E2E_HOST:-127.0.0.1}
source_image=${GITSSS_E2E_SOURCE_IMAGE:-gitea/gitea:1.26.2}
target_image=gitsss/gitsss:dev

cleanup() {
	docker rm -f "$source_name" "$target_name" >/dev/null 2>&1 || true
	docker volume rm "$source_volume" "$target_volume" >/dev/null 2>&1 || true
	docker network rm "$network_name" >/dev/null 2>&1 || true
	docker image rm "$source_image" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
cleanup

wait_health() {
	url=$1
	remaining=60
	while [ "$remaining" -gt 0 ]; do
		if [ "$(curl --silent --output /dev/null --write-out '%{http_code}' "$url")" = 200 ]; then
			return
		fi
		remaining=$((remaining - 1))
		sleep 1
	done
	echo "health timeout: $url" >&2
	exit 1
}

api() {
	method=$1
	url=$2
	token=$3
	body=${4-}
	if [ -n "$body" ]; then
		curl --fail --silent --show-error --request "$method" \
			--header "Authorization: token $token" --header 'Content-Type: application/json' \
			--data "$body" "$url"
	else
		curl --fail --silent --show-error --request "$method" \
			--header "Authorization: token $token" "$url"
	fi
}

if [ "${GITSSS_E2E_SKIP_BUILD:-0}" != 1 ]; then
	docker build --pull=false \
		--build-arg "NPM_REGISTRY=${GITSSS_E2E_NPM_REGISTRY:-https://registry.npmjs.org}" \
		--build-arg "GOPROXY=${GITSSS_E2E_GOPROXY:-https://proxy.golang.org,direct}" \
		--build-arg "UBUNTU_MIRROR=${GITSSS_E2E_UBUNTU_MIRROR:-}" \
		--build-arg "UBUNTU_PORTS_MIRROR=${GITSSS_E2E_UBUNTU_PORTS_MIRROR:-}" \
		--tag "$target_image" . >/dev/null
fi
docker image inspect "$target_image" >/dev/null
docker pull "$source_image" >/dev/null
docker network create "$network_name" >/dev/null
docker volume create "$source_volume" >/dev/null
docker volume create "$target_volume" >/dev/null

docker run --detach --name "$source_name" --network "$network_name" \
	--publish "127.0.0.1:${source_port}:3000" \
	--env USER_UID=1000 --env USER_GID=1000 \
	--env GITEA__database__DB_TYPE=sqlite3 \
	--env GITEA__server__ROOT_URL="http://${source_name}:3000/" \
	--env GITEA__server__LFS_START_SERVER=true \
	--env GITEA__security__INSTALL_LOCK=true \
	--env GITEA__service__DISABLE_REGISTRATION=true \
	--volume "${source_volume}:/data" "$source_image" >/dev/null

docker run --detach --name "$target_name" --network "$network_name" \
	--publish "127.0.0.1:${target_port}:3000" \
	--env USER_UID=1000 --env USER_GID=1000 \
	--env GITSSS__database__DB_TYPE=sqlite3 \
	--env GITSSS__server__ROOT_URL="http://${target_name}:3000/" \
	--env GITSSS__server__LFS_START_SERVER=true \
	--env GITSSS__security__INSTALL_LOCK=true \
	--env GITSSS__service__DISABLE_REGISTRATION=true \
	--env GITSSS__cron.update_checker__ENABLED=false \
	--volume "${target_volume}:/data" "$target_image" >/dev/null

wait_health "http://${host}:${source_port}/api/healthz"
wait_health "http://${host}:${target_port}/api/healthz"

source_output=$(docker exec --user git "$source_name" gitea admin user create \
	--username source-admin --password 'SourceAdminPass!233' --email source-admin@example.test \
	--admin --must-change-password=false --access-token --access-token-name http-import --access-token-scopes all 2>&1)
source_token=$(printf '%s\n' "$source_output" | grep -Eo '[0-9a-f]{40}' | tail -n 1)
test -n "$source_token"
docker exec --user git "$source_name" gitea admin user create --username alice --password 'AlicePass!233' --email alice@example.test --fullname 'Alice Developer' --must-change-password=false >/dev/null
docker exec --user git "$source_name" gitea admin user create --username bob --password 'BobPass!233' --email bob@example.test --fullname 'Bob Reviewer' --must-change-password=false >/dev/null
docker exec --user git "$source_name" gitea admin user create --username charlie --password 'CharliePass!233' --email charlie@example.test --fullname 'Charlie Guest' --must-change-password=false >/dev/null

source_api="http://${host}:${source_port}/api/v1"
api POST "$source_api/admin/users/alice/orgs" "$source_token" '{"username":"poke-game","full_name":"Poke Game","visibility":"private"}' >/dev/null
team_json=$(api POST "$source_api/orgs/poke-game/teams" "$source_token" '{"name":"Developers","description":"Write team","permission":"write","units":["repo.code"],"includes_all_repositories":false}')
team_id=$(printf '%s' "$team_json" | grep -Eo '"id":[0-9]+' | head -n 1 | cut -d: -f2)
test -n "$team_id"
api PUT "$source_api/teams/$team_id/members/alice" "$source_token" >/dev/null
api PUT "$source_api/teams/$team_id/members/bob" "$source_token" >/dev/null

for repo in data-collect-server-go config233-editor-server; do
	api POST "$source_api/orgs/poke-game/repos" "$source_token" "{\"name\":\"$repo\",\"private\":true,\"auto_init\":false}" >/dev/null
	api PUT "$source_api/teams/$team_id/repos/poke-game/$repo" "$source_token" >/dev/null
done
api PUT "$source_api/repos/poke-game/data-collect-server-go/collaborators/charlie" "$source_token" '{"permission":"read"}' >/dev/null

docker exec --user git "$source_name" sh -c 'set -eu
rm -rf /tmp/data-collect-server-go
mkdir /tmp/data-collect-server-go
cd /tmp/data-collect-server-go
git init -q
git config user.name "Fixture Author"
git config user.email fixture@example.test
printf "package main\n\nfunc main() {}\n" > main.go
git add main.go
git commit -q -m "initial data collector"
git branch -M main
git remote add origin "http://source-admin:SourceAdminPass%21233@127.0.0.1:3000/poke-game/data-collect-server-go.git"
git push -q -u origin main
printf "collector: enabled\n" > config.yaml
git add config.yaml
git commit -q -m "add collector config"
git tag -a v0.1.0 -m "fixture release"
git push -q origin main --tags
git switch -q -c feature/http-import
printf "http import\n" > migration.txt
git add migration.txt
git commit -q -m "add migration marker"
git push -q -u origin feature/http-import'

docker exec --user git "$source_name" sh -c 'set -eu
rm -rf /tmp/config233-editor-server
mkdir /tmp/config233-editor-server
cd /tmp/config233-editor-server
git init -q
git config user.name "Fixture Author"
git config user.email fixture@example.test
printf "{\"name\":\"config233\"}\n" > config.json
git add config.json
git commit -q -m "initial editor config"
git branch -M main
git remote add origin "http://source-admin:SourceAdminPass%21233@127.0.0.1:3000/poke-game/config233-editor-server.git"
git push -q -u origin main
printf "server: 3000\n" > app.yaml
git add app.yaml
git commit -q -m "add server config"
git tag v0.2.0
git push -q origin main --tags'

docker run --rm --network "$network_name" --entrypoint sh "$target_image" -c 'set -eu
git clone -q "http://source-admin:SourceAdminPass%21233@gitsss-http-source:3000/poke-game/config233-editor-server.git" /tmp/repo
cd /tmp/repo
git config user.name "Fixture LFS"
git config user.email fixture-lfs@example.test
git lfs install --local >/dev/null
git lfs track "*.bin" >/dev/null
dd if=/dev/urandom of=asset.bin bs=1024 count=16 status=none
git add .gitattributes asset.bin
git commit -q -m "add lfs asset"
git push -q origin main'

target_output=$(docker exec --user git "$target_name" gitsss admin user create \
	--username target-admin --password 'TargetAdminPass!233' --email target-admin@example.test \
	--admin --must-change-password=false --access-token --access-token-name http-import --access-token-scopes all 2>&1)
target_token=$(printf '%s\n' "$target_output" | grep -Eo '[0-9a-f]{40}' | tail -n 1)
test -n "$target_token"
docker exec --user root "$target_name" sh -c "set -eu
mkdir -p /data/e2e
umask 077
printf %s '$source_token' > /data/e2e/source.token
printf %s '$target_token' > /data/e2e/target.token
chown -R 1000:1000 /data/e2e"

import_args="import-gitea-http --source-url http://${source_name}:3000 --source-token-file /data/e2e/source.token --target-url http://${target_name}:3000 --target-token-file /data/e2e/target.token --repo poke-game/data-collect-server-go --repo poke-game/config233-editor-server"
dry_run=$(docker run --rm --network "$network_name" --volume "${target_volume}:/data" --entrypoint /app/gitsss/gitsss "$target_image" $import_args)
printf '%s' "$dry_run" | grep -q '"repositories": \['
target_before=$(api GET "http://${host}:${target_port}/api/v1/admin/users?limit=50" "$target_token" | grep -o '"login"' | wc -l | tr -d ' ')
test "$target_before" -eq 1

result=$(docker run --rm --network "$network_name" --volume "${target_volume}:/data" --entrypoint /app/gitsss/gitsss "$target_image" \
	$import_args --apply --credentials-file /data/e2e/credentials.jsonl)
printf '%s' "$result" | grep -q '"users_created": 4'
printf '%s' "$result" | grep -q '"repositories_created": 2'
printf '%s' "$result" | grep -q '"refs_verified": 5'

target_api="http://${host}:${target_port}/api/v1"
api GET "$target_api/orgs/poke-game/teams" "$target_token" | grep -q '"repo.code":"write"'
api GET "$target_api/repos/poke-game/data-collect-server-go/collaborators/charlie/permission" "$target_token" | grep -q '"permission":"read"'
docker exec "$target_name" sqlite3 /data/gitsss/gitsss.db "select name from user where must_change_password=1 and name in ('alice','bob','charlie','source-admin');" | grep -q source-admin
test "$(docker exec "$target_name" sh -c "stat -c %a /data/e2e/credentials.jsonl")" = 600
test "$(docker exec "$target_name" sh -c "wc -l < /data/e2e/credentials.jsonl")" = 4

docker run --rm --network "$network_name" --entrypoint sh "$target_image" -c 'set -eu
for repo in data-collect-server-go config233-editor-server; do
  git ls-remote "http://source-admin:SourceAdminPass%21233@gitsss-http-source:3000/poke-game/$repo.git" | grep -E "refs/(heads|tags|notes)/" | sort > /tmp/source-refs
  git ls-remote "http://target-admin:TargetAdminPass%21233@gitsss-http-target:3000/poke-game/$repo.git" | grep -E "refs/(heads|tags|notes)/" | sort > /tmp/target-refs
  diff -u /tmp/source-refs /tmp/target-refs
done
git clone -q "http://target-admin:TargetAdminPass%21233@gitsss-http-target:3000/poke-game/config233-editor-server.git" /tmp/check
test "$(wc -c < /tmp/check/asset.bin)" -eq 16384'

echo "$result"
echo "Gitea HTTP migration E2E passed: accounts=4 orgs=1 repos=2 refs=5 LFS=1"
