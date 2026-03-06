package errs

import "errors"

type Code string

type Error struct {
	Code    Code
	Message string
	Err     error
	Fields  map[string]string
	Details any
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func WithFields(err error, fields map[string]string) error {
	if len(fields) == 0 {
		return err
	}
	var typed *Error
	if errors.As(err, &typed) {
		if typed.Fields == nil {
			typed.Fields = map[string]string{}
		}
		for k, v := range fields {
			typed.Fields[k] = v
		}
		return typed
	}
	return &Error{Code: CodeValidationFields, Message: "validation failed", Err: err, Fields: fields}
}

func WithDetails(err error, details any) error {
	if details == nil {
		return err
	}
	var typed *Error
	if errors.As(err, &typed) {
		typed.Details = details
		return typed
	}
	return &Error{Code: CodeInternal, Message: "unexpected error", Err: err, Details: details}
}

func From(err error) (*Error, bool) {
	var typed *Error
	if errors.As(err, &typed) {
		return typed, true
	}
	return nil, false
}

const (
	CodeInternal            Code = "CORE-500"
	CodeBadRequest          Code = "CORE-400"
	CodeNotFound            Code = "CORE-404"
	CodeValidationFields    Code = "VAL-400-FIELDS"
	CodeValidationName      Code = "VAL-400-NAME"
	CodeValidationSubdomain Code = "VAL-400-SUBDOMAIN"
	CodeValidationPort      Code = "VAL-400-PORT"
	CodeValidationDomain    Code = "VAL-400-DOMAIN"
	CodeDomainMissing       Code = "VAL-400-DOMAIN-MISSING"
	CodeDomainNotConfigured Code = "VAL-400-DOMAIN-NOT-CONFIGURED"
	CodeContainerName       Code = "VAL-400-CONTAINER-NAME"

	CodeAuthUnauthenticated    Code = "AUTH-401"
	CodeAuthForbidden          Code = "AUTH-403"
	CodeAuthAdminRequired      Code = "AUTH-403-ADMIN"
	CodeAuthStateGenerate      Code = "AUTH-500-STATE"
	CodeAuthCallbackMissing    Code = "AUTH-400-CALLBACK"
	CodeAuthStateInvalid       Code = "AUTH-400-STATE"
	CodeAuthLoginFailed        Code = "AUTH-500-LOGIN"
	CodeAuthSessionCreate      Code = "AUTH-500-SESSION"
	CodeAuthTestTokenInvalid   Code = "AUTH-400-TEST-TOKEN"
	CodeAuthTestTokenDisabled  Code = "AUTH-404-TEST-TOKEN"
	CodeAuthInvalidCredentials Code = "AUTH-401-CREDENTIALS"

	CodeJobInvalidID         Code = "JOB-400-ID"
	CodeJobInvalidBody       Code = "JOB-400-BODY"
	CodeJobNotFound          Code = "JOB-404"
	CodeJobAlreadyFinished   Code = "JOB-409-FINISHED"
	CodeJobRunning           Code = "JOB-409-RUNNING"
	CodeJobNotStoppable      Code = "JOB-409-NOT-STOPPABLE"
	CodeJobNotRetryable      Code = "JOB-409-NOT-RETRYABLE"
	CodeJobListFailed        Code = "JOB-500-LIST"
	CodeJobStopFailed        Code = "JOB-500-STOP"
	CodeJobRetryFailed       Code = "JOB-500-RETRY"
	CodeJobStreamUnsupported Code = "JOB-500-STREAM"

	CodeHostInvalidProject    Code = "HOST-400-PROJECT"
	CodeHostInvalidContainer  Code = "HOST-400-CONTAINER"
	CodeHostInvalidBody       Code = "HOST-400-BODY"
	CodeHostAdminRequired     Code = "HOST-403-ADMIN"
	CodeHostDockerFailed      Code = "HOST-500-DOCKER"
	CodeHostUsageFailed       Code = "HOST-500-USAGE"
	CodeHostLogsFailed        Code = "HOST-500-LOGS"
	CodeHostStreamUnsupported Code = "HOST-500-STREAM"

	CodeSettingsLoadFailed    Code = "SETTINGS-500-LOAD"
	CodeSettingsSourcesFailed Code = "SETTINGS-500-SOURCES"
	CodeSettingsAdminRequired Code = "SETTINGS-403-ADMIN"
	CodeSettingsInvalidBody   Code = "SETTINGS-400-BODY"
	CodeSettingsUpdateFailed  Code = "SETTINGS-500-UPDATE"
	CodeSettingsPreviewFailed Code = "SETTINGS-400-PREVIEW"
	CodeSettingsSyncFailed    Code = "SETTINGS-500-SYNC"

	CodeCloudflareUnavailable    Code = "CF-500-SERVICE"
	CodeCloudflarePreflight      Code = "CF-502-PREFLIGHT"
	CodeCloudflareZones          Code = "CF-502-ZONES"
	CodeCloudflareMissingToken   Code = "CF-400-TOKEN"
	CodeCloudflareMissingAccount Code = "CF-400-ACCOUNT"
	CodeCloudflareMissingZone    Code = "CF-400-ZONE"
	CodeCloudflareMissingTunnel  Code = "CF-400-TUNNEL"
	CodeCloudflareTunnelLocal    Code = "CF-409-TUNNEL-LOCAL"

	CodeNetBirdUnavailable     Code = "NETBIRD-500-SERVICE"
	CodeNetBirdStatusFailed    Code = "NETBIRD-500-STATUS"
	CodeNetBirdACLGraphFailed  Code = "NETBIRD-500-ACL-GRAPH"
	CodeNetBirdPlanFailed      Code = "NETBIRD-500-PLAN"
	CodeNetBirdApplyFailed     Code = "NETBIRD-500-APPLY"
	CodeNetBirdReapplyFailed   Code = "NETBIRD-500-REAPPLY"
	CodeNetBirdReconcileFailed Code = "NETBIRD-500-RECONCILE"
	CodeNetBirdInvalidBody     Code = "NETBIRD-400-BODY"
	CodeNetBirdInvalidMode     Code = "NETBIRD-400-MODE"
	CodeNetBirdAdminRequired   Code = "NETBIRD-403-ADMIN"

	CodeGitHubUnavailable  Code = "GH-500-SERVICE"
	CodeGitHubCatalog      Code = "GH-500-CATALOG"
	CodeGitHubMissingToken Code = "GH-400-TOKEN"

	CodeUserInvalidID       Code = "USER-400-ID"
	CodeUserInvalidPayload  Code = "USER-400-PAYLOAD"
	CodeUserInvalidRole     Code = "USER-400-ROLE"
	CodeUserLoginRequired   Code = "USER-400-LOGIN"
	CodeUserNotFound        Code = "USER-404"
	CodeUserLastSuperUser   Code = "USER-400-LAST-SUPERUSER"
	CodeUserGitHubNotFound  Code = "USER-404-GITHUB"
	CodeUserRemoveSuperUser Code = "USER-400-SUPERUSER"
	CodeUserListFailed      Code = "USER-500-LIST"
	CodeUserUpdateFailed    Code = "USER-500-UPDATE"
	CodeUserCreateFailed    Code = "USER-500-CREATE"
	CodeUserDeleteFailed    Code = "USER-500-DELETE"

	CodeProjectListFailed              Code = "PROJECT-500-LIST"
	CodeProjectLocalListFailed         Code = "PROJECT-500-LOCAL"
	CodeProjectInvalidBody             Code = "PROJECT-400-BODY"
	CodeProjectAdminRequired           Code = "PROJECT-403-ADMIN"
	CodeProjectCreateFailed            Code = "PROJECT-400-CREATE"
	CodeProjectDeployFailed            Code = "PROJECT-400-DEPLOY"
	CodeProjectForwardFailed           Code = "PROJECT-400-FORWARD"
	CodeProjectQuickFailed             Code = "PROJECT-400-QUICK"
	CodeProjectTemplateSource          Code = "PROJECT-400-TEMPLATE"
	CodeProjectInvalidName             Code = "PROJECT-400-NAME"
	CodeProjectInvalidContainer        Code = "PROJECT-400-CONTAINER"
	CodeProjectNotFound                Code = "PROJECT-404"
	CodeProjectContainerNotFound       Code = "PROJECT-404-CONTAINER"
	CodeProjectDetailFailed            Code = "PROJECT-500-DETAIL"
	CodeProjectJobsFailed              Code = "PROJECT-500-JOBS"
	CodeProjectWorkbenchImportFailed   Code = "PROJECT-500-WB-IMPORT"
	CodeProjectWorkbenchPreviewFailed  Code = "PROJECT-500-WB-PREVIEW"
	CodeProjectWorkbenchApplyFailed    Code = "PROJECT-500-WB-APPLY"
	CodeProjectWorkbenchRestoreFailed  Code = "PROJECT-500-WB-RESTORE"
	CodeProjectArchivePlanFailed       Code = "PROJECT-500-ARCHIVE-PLAN"
	CodeProjectArchiveFailed           Code = "PROJECT-500-ARCHIVE"
	CodeProjectStackFailed             Code = "PROJECT-500-STACK"
	CodeProjectContainerFailed         Code = "PROJECT-500-CONTAINER"
	CodeProjectLogsFailed              Code = "PROJECT-500-LOGS"
	CodeProjectEnvReadFailed           Code = "PROJECT-500-ENV-READ"
	CodeProjectEnvWriteFailed          Code = "PROJECT-500-ENV-WRITE"
	CodeProjectEnvTooLarge             Code = "PROJECT-400-ENV-SIZE"
	CodeWorkbenchSourceNotFound        Code = "WB-404-SOURCE"
	CodeWorkbenchSourceInvalid         Code = "WB-400-SOURCE"
	CodeWorkbenchLocked                Code = "WB-409-LOCKED"
	CodeWorkbenchStaleRevision         Code = "WB-409-STALE-REVISION"
	CodeWorkbenchDriftDetected         Code = "WB-409-DRIFT-DETECTED"
	CodeWorkbenchValidationFailed      Code = "WB-422-VALIDATION"
	CodeWorkbenchGenerateFailed        Code = "WB-500-GENERATE"
	CodeWorkbenchStorageFailed         Code = "WB-500-STORAGE"
	CodeWorkbenchBackupNotFound        Code = "WB-404-BACKUP"
	CodeWorkbenchBackupIntegrity       Code = "WB-409-BACKUP-INTEGRITY"
	CodeWorkbenchBackupWriteFailed     Code = "WB-500-BACKUP-WRITE"
	CodeWorkbenchBackupRetentionFailed Code = "WB-500-BACKUP-RETENTION"
	CodeWorkbenchRestoreFailed         Code = "WB-500-RESTORE"

	CodeRBACSuperUserCap Code = "RBAC-409-SUPERUSER-CAP"

	CodeAuditListFailed Code = "AUDIT-500-LIST"
)
