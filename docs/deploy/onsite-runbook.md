# First-Customer On-Site Runbook

## 1. 사전 준비 / Prerequisites
한국어:
- 대상 서버에는 Docker Engine `24.0+`가 설치되어 있어야 합니다.
- 대상 서버에서 외부 `HTTPS (443)` 아웃바운드 연결이 허용되어야 합니다.
- 에이전트가 실행되는 서버에서 고객 MySQL에 접근 가능해야 합니다.
- 가능하면 운영 본 DB 대신 읽기 전용 replica를 사용합니다. replica가 없으면 read-only 사용자로 본 DB에 연결하고 배포 로그에 그 사실을 기록합니다.
- 컨트롤 플레인 URL, 테넌트 토큰, 작업 공간 슬러그를 미리 확인합니다.
- 고객 서버에 최소 두 개의 영구 디렉터리를 만들 수 있어야 합니다.
  `/etc/<workspace-slug>-agent`
  `/var/lib/<workspace-slug>-agent`

English:
- The target server must have Docker Engine `24.0+` installed.
- Outbound `HTTPS (443)` from the target server must be allowed.
- The host that runs the agent must be able to reach the customer MySQL server.
- Prefer a read replica over the primary database when one is available. If no replica exists, connect to the primary with a read-only user and note that in the deploy log.
- Confirm the control-plane URL, tenant token, and workspace slug before you begin.
- The customer server must allow at least two persistent directories.
  `/etc/<workspace-slug>-agent`
  `/var/lib/<workspace-slug>-agent`

## 2. 작업 공간 준비 / Prepare The Workspace
한국어:
1. 제품에 로그인하고 해당 고객 작업 공간을 엽니다.
2. 온보딩 1단계에서 작업 공간 이름을 최종 확인합니다.
3. 온보딩 2단계에서 표시된 Docker 명령을 복사합니다.
4. 고객 서버에서 아래처럼 디렉터리를 먼저 만듭니다.

```bash
sudo mkdir -p /etc/<workspace-slug>-agent /var/lib/<workspace-slug>-agent
sudo chown "$USER":"$USER" /etc/<workspace-slug>-agent /var/lib/<workspace-slug>-agent
```

English:
1. Sign in to the product and open the customer workspace.
2. Confirm the final workspace name in onboarding step 1.
3. Copy the Docker command shown in onboarding step 2.
4. Create the persistent directories on the customer host first.

```bash
sudo mkdir -p /etc/<workspace-slug>-agent /var/lib/<workspace-slug>-agent
sudo chown "$USER":"$USER" /etc/<workspace-slug>-agent /var/lib/<workspace-slug>-agent
```

## 3. 에이전트 컨테이너 실행 / Run The Agent Container
한국어:
- 온보딩에서 복사한 명령을 그대로 사용합니다. 아래 형식이 맞아야 합니다.

```bash
docker run -d --name <workspace-slug>-agent \
  --restart unless-stopped \
  -e CONTROL_PLANE_URL=https://control-plane.example.com \
  -e TENANT_TOKEN=<tenant-token> \
  -e AGENT_VERSION=v0.1.0 \
  -v /etc/<workspace-slug>-agent:/etc/agent \
  -v /var/lib/<workspace-slug>-agent:/var/lib/agent \
  registry.digitalocean.com/mission/edge-agent:v0.1.0
```

- 컨테이너가 올라오면 온보딩 화면이 자동으로 연결 상태를 감지해야 합니다.
- 연결이 되지 않으면 먼저 아래를 확인합니다.
  `docker logs <workspace-slug>-agent`
  `curl -I https://control-plane.example.com/healthz`

English:
- Use the command copied from onboarding exactly as shown. The shape should match this:

```bash
docker run -d --name <workspace-slug>-agent \
  --restart unless-stopped \
  -e CONTROL_PLANE_URL=https://control-plane.example.com \
  -e TENANT_TOKEN=<tenant-token> \
  -e AGENT_VERSION=v0.1.0 \
  -v /etc/<workspace-slug>-agent:/etc/agent \
  -v /var/lib/<workspace-slug>-agent:/var/lib/agent \
  registry.digitalocean.com/mission/edge-agent:v0.1.0
```

- Once the container starts, the onboarding UI should detect the agent connection automatically.
- If it does not connect, check these first.
  `docker logs <workspace-slug>-agent`
  `curl -I https://control-plane.example.com/healthz`

## 4. MySQL 읽기 전용 계정 생성 / Create The Read-Only MySQL User
한국어:
- 고객 MySQL에 접속해서 아래 SQL을 고객 환경 값으로 바꿔 붙여 넣습니다.
- 가능하면 replica에서 실행합니다.

```sql
CREATE USER 'okta_ai_ro'@'%' IDENTIFIED BY '<strong-random-password>';
GRANT SELECT, SHOW VIEW ON `<database_name>`.* TO 'okta_ai_ro'@'%';
FLUSH PRIVILEGES;
```

- 생성 후 DSN 예시는 아래 형식입니다.

```text
okta_ai_ro:<strong-random-password>@tcp(<mysql-host>:3306)/<database_name>
```

English:
- Connect to the customer MySQL instance and paste this SQL after replacing the customer-specific values.
- Run it on the replica when one is available.

```sql
CREATE USER 'okta_ai_ro'@'%' IDENTIFIED BY '<strong-random-password>';
GRANT SELECT, SHOW VIEW ON `<database_name>`.* TO 'okta_ai_ro'@'%';
FLUSH PRIVILEGES;
```

- The DSN should look like this after the user exists.

```text
okta_ai_ro:<strong-random-password>@tcp(<mysql-host>:3306)/<database_name>
```

## 5. 제품에 MySQL 연결 정보 입력 / Point The Product At MySQL
한국어:
1. 온보딩 3단계에서 MySQL host, port, database name을 입력합니다.
2. 위에서 만든 DSN을 연결 문자열 칸에 붙여 넣습니다.
3. 연결 테스트가 성공하면 제품이 다음 단계로 자동 이동합니다.
4. 실패하면 고객 방화벽, 계정 권한, replica 접근 경로를 먼저 확인합니다.

English:
1. In onboarding step 3, enter the MySQL host, port, and database name.
2. Paste the DSN from the previous step into the connection-string field.
3. When the connection test succeeds, the product should advance automatically.
4. If it fails, check the firewall path, read-only grants, and replica reachability first.

## 6. 스키마 캡처 및 시맨틱 레이어 승인 / Capture Schema And Approve The Semantic Layer
한국어:
1. 온보딩 4단계에서 스키마 캡처를 실행합니다.
2. 테이블 수와 컬럼 수가 실제 DB와 대략 맞는지 확인합니다.
3. 온보딩 5단계에서 생성된 시맨틱 레이어 초안을 검토합니다.
4. 고객이 직접 읽어 보고 승인 버튼을 누르게 합니다.
5. 승인 후 starter questions 단계까지 진행합니다.

English:
1. Run schema capture in onboarding step 4.
2. Confirm that the table and column counts look roughly correct for the real database.
3. Review the generated semantic-layer draft in onboarding step 5.
4. Have the customer read it and press approve personally.
5. Continue through the starter-questions step after approval.

## 7. 로컬 감사 로그 확인 / Verify The Local Audit Log
한국어:
- 고객이 직접 질문을 최소 3개 입력하게 하고, 서버에서 로컬 감사 로그를 확인합니다.
- 로그에는 SQL, DB 이름, 실행 시간, 행 수, 차단 사유는 있어야 하지만 raw rows는 없어야 합니다.

```bash
tail -n 20 /var/lib/<workspace-slug>-agent/audit/query-events.jsonl
```

- 확인 포인트:
  `database_name` 이 고객 DB인지
  `sql` 이 질문과 맞는지
  `row_count` 와 `elapsed_ms` 가 기록되는지
  결과 row payload 자체는 기록되지 않는지

English:
- Have the customer ask at least 3 questions on their own, then inspect the local audit log on the host.
- The log should contain SQL, database name, elapsed time, row count, and blocked reasons when applicable, but it must not contain raw rows.

```bash
tail -n 20 /var/lib/<workspace-slug>-agent/audit/query-events.jsonl
```

- Confirm these points:
  `database_name` matches the customer database
  `sql` matches the user’s question
  `row_count` and `elapsed_ms` are present
  no raw result-row payload is recorded

## 8. 현장 완료 체크 / Exit Criteria On Site
한국어:
- 고객이 도움 없이 온보딩을 끝까지 완료했는지 확인합니다.
- 고객이 직접 최소 3개의 질문을 던졌는지 확인합니다.
- 에이전트 로그에서 쿼리가 로컬에서 실행됐는지 확인합니다.
- 컨트롤 플레인 로그에는 raw row data가 없는지 확인합니다.
- 첫 주간 체크인 일정을 7일 뒤로 잡습니다.

English:
- Confirm that the customer completed onboarding without prompting.
- Confirm that the customer asked at least 3 questions personally.
- Verify from the agent-side log that queries ran locally.
- Verify that control-plane logs do not contain raw row data.
- Schedule the first weekly check-in for 7 days later.

## 9. 롤백 / Rollback
한국어:
1. 컨테이너를 중지하고 삭제합니다.
2. 제품에서 해당 테넌트 토큰을 revoke 합니다.
3. MySQL에서 읽기 전용 계정을 삭제합니다.

```bash
docker rm -f <workspace-slug>-agent
```

```sql
DROP USER 'okta_ai_ro'@'%';
FLUSH PRIVILEGES;
```

English:
1. Stop and remove the container.
2. Revoke the tenant token from the product.
3. Drop the read-only MySQL user.

```bash
docker rm -f <workspace-slug>-agent
```

```sql
DROP USER 'okta_ai_ro'@'%';
FLUSH PRIVILEGES;
```
