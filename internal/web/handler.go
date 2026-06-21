package web

import (
	"html/template"

	"file-storage-server/internal/access"
	"file-storage-server/internal/admin"
	"file-storage-server/internal/audit"
	"file-storage-server/internal/auth"
	"file-storage-server/internal/file"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/metrics"
	"file-storage-server/internal/profile"
	"file-storage-server/internal/share"
	"file-storage-server/internal/storage"
)

type Handler struct {
	auth     *auth.Service
	storages *storage.Service
	files    *file.Service
	folders  *folder.Service
	accesses *access.StorageAccessService
	admin    *admin.Service
	metrics  *metrics.Registry
	profile  *profile.Service
	shares   *share.Service
	audit    *audit.Service
	fileRoot string
	tpl      *template.Template
}

type PageData struct {
	Title            string
	User             *auth.CurrentUser
	Error            string
	Flash            *Flash
	Storage          *storage.StorageDTO
	Storages         []storage.StorageDTO
	FolderID         *int64
	Folder           *folder.FolderDTO
	Folders          []folder.FolderDTO
	Path             []folder.FolderDTO
	Files            []file.FileDTO
	FileSearch       string
	FileSort         string
	FileDirection    string
	FilePage         int
	FilePrevPage     int
	FileNextPage     int
	FileTotal        int
	FileHasPrev      bool
	FileHasNext      bool
	Accesses         []access.StorageAccessDTO
	FolderAccesses   map[int64][]access.FolderAccessDTO
	FileLinks        map[int64][]share.ShareLinkDTO
	Admin            *admin.Dashboard
	Metrics          metrics.Snapshot
	Profile          *profile.Profile
	File             *file.FileDTO
	Links            []share.ShareLinkDTO
	ShareURL         string
	ActiveLinkFileID int64
	Theme            string
	Language         string
	CurrentURL       string
}

type Flash struct {
	Kind    string
	Message string
}

func NewHandler(
	authService *auth.Service,
	storageService *storage.Service,
	fileService *file.Service,
	folderService *folder.Service,
	accessService *access.StorageAccessService,
	adminService *admin.Service,
	metricsRegistry *metrics.Registry,
	profileService *profile.Service,
	shareService *share.Service,
	auditService *audit.Service,
	fileRoot string,
	templatePattern string,
) (*Handler, error) {
	tpl, err := template.New("").Funcs(template.FuncMap{
		"formatBytes":            formatBytes,
		"usagePercent":           usagePercent,
		"displayAccess":          displayAccess,
		"displayAccessLang":      displayAccessLang,
		"displayShare":           displayShare,
		"displayShareLang":       displayShareLang,
		"displayVisibility":      displayVisibility,
		"displayVisibilityLang":  displayVisibilityLang,
		"displayStorageType":     displayStorageType,
		"displayStorageTypeLang": displayStorageTypeLang,
		"displayFileType":        displayFileType,
		"displayFileTypeLang":    displayFileTypeLang,
		"formatDateDisplay":      formatDateDisplay,
		"formatDateInput":        formatDateInput,
		"formatDuration":         formatDuration,
		"formatSeconds":          formatSeconds,
		"t":                      translate,
	}).ParseGlob(templatePattern)
	if err != nil {
		return nil, err
	}

	return &Handler{
		auth:     authService,
		storages: storageService,
		files:    fileService,
		folders:  folderService,
		accesses: accessService,
		admin:    adminService,
		metrics:  metricsRegistry,
		profile:  profileService,
		shares:   shareService,
		audit:    auditService,
		fileRoot: fileRoot,
		tpl:      tpl,
	}, nil
}
