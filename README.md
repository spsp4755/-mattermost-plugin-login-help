# Mattermost Login Help Mailer

Mattermost audit log를 읽어서 로그인 실패가 반복된 사용자에게
Confluence 비밀번호 초기화 안내 페이지를 메일로 보내는 서버 플러그인입니다.

## 동작 방식

1. Mattermost JSON audit log에서 `event_name=login`, `status=fail` 이벤트를 읽습니다.
2. 사용자별 실패 횟수를 KV Store에 저장합니다.
3. 설정한 시간 창(`WindowMinutes`) 안에서 실패 횟수가 기준값(`FailureThreshold`) 이상이면 메일을 보냅니다.
4. 사용자가 나중에 성공 로그인하면 저장한 실패 카운트를 초기화합니다.

## 권장 운영 조건

- Mattermost audit logging을 활성화하세요.
- audit log 출력 형식은 JSON이 좋습니다.
- 플러그인 프로세스가 audit log 파일을 읽을 수 있어야 합니다.
- `StartFromEnd=true`로 시작하면 과거 로그를 재처리하지 않습니다.
- `OnlyLocalAccounts=true`로 두면 LDAP/SSO 계정은 제외됩니다.

## 한계와 주의점

- 이 버전은 단일 노드 또는 중앙 공유 audit log 파일을 기준으로 설계했습니다.
- HA 환경에서 노드별 로컬 로그가 분리되어 있으면 외부 watcher 또는 중앙 로그 수집과 함께 쓰는 편이 더 안전합니다.
- Mattermost audit log에 실패한 사용자를 식별할 정보가 없으면 해당 이벤트는 건너뜁니다.
- 실제 비밀번호를 바꾸는 플러그인이 아니라, 재설정 방법이 적힌 Confluence 링크를 자동 안내하는 플러그인입니다.

## 관리자 API

- 상태 확인: `GET /plugins/com.mattermost.login-help-mailer/api/v1/status`
- 테스트 메일: `POST /plugins/com.mattermost.login-help-mailer/api/v1/test-mail`

두 API 모두 Mattermost 시스템 관리자 로그인 세션이 필요합니다.

## 빌드

```bash
make bundle
```

결과물은 `dist/com.mattermost.login-help-mailer-0.1.0.tar.gz`에 생성됩니다.

Windows PowerShell 환경에서는 아래 스크립트를 써도 됩니다.

```powershell
.\build.ps1
```
