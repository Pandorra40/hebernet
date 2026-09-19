package protocol

// Request is the JSON envelope sent over the Unix socket.
type Request struct {
	ID     string         `json:"id"`
	Op     string         `json:"op"`
	Params map[string]any `json:"params,omitempty"`
}

// Response is the JSON envelope returned by the agent.
type Response struct {
	ID      string         `json:"id"`
	OK      bool           `json:"ok"`
	Error   string         `json:"error,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
}

const (
	OpPing           = "ping"
	OpCreateSiteUser = "create_site_user"
	OpDeleteSiteUser = "delete_site_user"
	OpSetQuota       = "set_quota"
	OpProvisionNginx = "provision_nginx"
	OpProvisionPHP   = "provision_php"
	OpProvisionWP         = "provision_wordpress" // legacy
	OpProvisionLaravel    = "provision_laravel"   // legacy
	OpProvisionPrestaShop = "provision_prestashop" // legacy
	OpProvisionBludit     = "provision_bludit"
	OpProvisionHugo       = "provision_hugo"
	OpProvisionCodeIgniter = "provision_codeigniter"
	OpIssueSSL            = "issue_ssl"
	OpSuspendSite    = "suspend_site"
	OpResumeSite     = "resume_site"
	OpResetSFTPPass  = "reset_sftp_password"
	OpCreateDatabase = "create_database"
	OpDestroySite    = "destroy_site"
	OpQuotaUsage     = "quota_usage"
	OpReadLogs       = "read_logs"
	OpResetDBPass    = "reset_db_password"
	OpSyncCron       = "sync_cron"
	OpInstallAdminer = "install_adminer"
	OpEnsureFM       = "ensure_filebrowser"
)
