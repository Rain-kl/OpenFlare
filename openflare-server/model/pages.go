package model

import "time"

const (
	PagesDeploymentStatusUploaded = "uploaded"
	PagesDeploymentStatusActive   = "active"
)

type PagesProject struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	Name               string    `json:"name" gorm:"size:255;not null"`
	Slug               string    `json:"slug" gorm:"uniqueIndex;size:128;not null"`
	Description        string    `json:"description" gorm:"type:text;not null;default:''"`
	Enabled            bool      `json:"enabled" gorm:"not null;default:true"`
	SPAFallbackEnabled bool      `json:"spa_fallback_enabled" gorm:"not null;default:false"`
	SPAFallbackPath    string    `json:"spa_fallback_path" gorm:"size:512;not null;default:'/index.html'"`
	APIProxyEnabled    bool      `json:"api_proxy_enabled" gorm:"not null;default:false"`
	APIProxyPath       string    `json:"api_proxy_path" gorm:"size:255;not null;default:''"`
	APIProxyPass       string    `json:"api_proxy_pass" gorm:"size:2048;not null;default:''"`
	APIProxyRewrite    string    `json:"api_proxy_rewrite" gorm:"size:255;not null;default:''"`
	ActiveDeploymentID *uint     `json:"active_deployment_id" gorm:"index"`
	RootDir            string    `json:"root_dir" gorm:"size:512;not null;default:''"`
	EntryFile          string    `json:"entry_file" gorm:"size:512;not null;default:'index.html'"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type PagesDeployment struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	ProjectID        uint       `json:"project_id" gorm:"not null;index"`
	DeploymentNumber int        `json:"deployment_number" gorm:"not null"`
	Checksum         string     `json:"checksum" gorm:"size:64;not null;index"`
	Status           string     `json:"status" gorm:"size:32;not null;default:'uploaded';index"`
	ArtifactPath     string     `json:"artifact_path" gorm:"size:2048;not null"`
	FileCount        int        `json:"file_count" gorm:"not null;default:0"`
	TotalSize        int64      `json:"total_size" gorm:"not null;default:0"`
	CreatedBy        string     `json:"created_by" gorm:"size:64;not null;default:''"`
	CreatedAt        time.Time  `json:"created_at"`
	ActivatedAt      *time.Time `json:"activated_at"`
}

type PagesDeploymentFile struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	DeploymentID uint      `json:"deployment_id" gorm:"not null;index"`
	Path         string    `json:"path" gorm:"size:2048;not null"`
	Size         int64     `json:"size" gorm:"not null;default:0"`
	Checksum     string    `json:"checksum" gorm:"size:64;not null"`
	CreatedAt    time.Time `json:"created_at"`
}

func ListPagesProjects() (projects []*PagesProject, err error) {
	err = DB.Order("id desc").Find(&projects).Error
	return projects, err
}

func GetPagesProjectByID(id uint) (*PagesProject, error) {
	project := &PagesProject{}
	err := DB.First(project, id).Error
	return project, err
}

func GetPagesProjectBySlug(slug string) (*PagesProject, error) {
	project := &PagesProject{}
	err := DB.Where("slug = ?", slug).First(project).Error
	return project, err
}

func ListPagesDeployments(projectID uint) (deployments []*PagesDeployment, err error) {
	err = DB.Where("project_id = ?", projectID).Order("id desc").Find(&deployments).Error
	return deployments, err
}

func GetPagesDeploymentByID(id uint) (*PagesDeployment, error) {
	deployment := &PagesDeployment{}
	err := DB.First(deployment, id).Error
	return deployment, err
}

func ListPagesDeploymentFiles(deploymentID uint) (files []*PagesDeploymentFile, err error) {
	err = DB.Where("deployment_id = ?", deploymentID).Order("path asc").Find(&files).Error
	return files, err
}
