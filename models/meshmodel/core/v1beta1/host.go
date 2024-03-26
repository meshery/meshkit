package v1beta1

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/utils/kubernetes"
	"gorm.io/gorm"
)

var hostCreationLock sync.Mutex //Each entity will perform a check and if the host already doesn't exist, it will create a host. This lock will make sure that there are no race conditions.

type Host struct {
	ID        uuid.UUID `json:"-"`
	Hostname  string
	Port      int
	Metadata  string
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	IHost     IHost    `gorm:"-"`
}

type MeshModelHostsWithEntitySummary struct {
	ID       uuid.UUID     `json:"id"`
	Hostname string        `json:"hostname"`
	Port     int           `json:"port"`
	Summary  EntitySummary `json:"summary"`
}
type EntitySummary struct {
	Models        int64 `json:"models"`
	Components    int64 `json:"components"`
	Relationships int64 `json:"relationships"`
	Policies      int64 `json:"policies"`
}
type MesheryHostSummaryDB struct {
	HostID        uuid.UUID `json:"-" gorm:"id"`
	Hostname      string    `json:"-" gorm:"hostname"`
	Port          int       `json:"-" gorm:"port"`
	Models        int64     `json:"-" gorm:"models"`
	Components    int64     `json:"-" gorm:"components"`
	Relationships int64     `json:"-" gorm:"relationships"`
	Policies      int64     `json:"-" gorm:"policies"`
}

type HostFilter struct {
	Name        string
	Greedy      bool //when set to true - instead of an exact match, name will be prefix matched
	Trim        bool //when set to true - the schema is not returned
	DisplayName string
	Version     string
	Sort        string //asc or desc. Default behavior is asc
	OrderOn     string
	Limit       int //If 0 or  unspecified then all records are returned and limit is not used
	Offset      int
}

func (h *Host) Create(db *database.Handler) (uuid.UUID, error) {
	byt, err := json.Marshal(h)
	if err != nil {
		return uuid.UUID{}, err
	}
	hID := uuid.NewSHA1(uuid.UUID{}, byt)
	var host Host
	hostCreationLock.Lock()
	defer hostCreationLock.Unlock()
	err = db.First(&host, "id = ?", hID).Error // check if the host already exists
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.UUID{}, err
	}

	// if not exists then create a new host and return the id
	if err == gorm.ErrRecordNotFound {
		h.ID = hID
		err = db.Create(&h).Error
		if err != nil {
			return uuid.UUID{}, err
		}
		return h.ID, nil
	}

	// else return the id of the existing host
	return host.ID, nil
}

func (h *Host) AfterFind(tx *gorm.DB) error {
	switch h.Hostname {
	case "artifacthub":
		h.IHost = ArtifactHub{}
	case "kubernetes":
		h.IHost = Kubernetes{}
	default: // do nothing if the host is not pre-unknown. Currently adapters fall into this case.
		return nil
	}
	return nil
}

// Each host from where meshmodels can be generated needs to implement this interface
// HandleDependents, contains host specific logic for provisioning required CRDs/operators for corresponding components.
type IHost interface {
	HandleDependents(comp Component, kc *kubernetes.Client, isDeploy bool) (string, error)
	String() string
}

type ArtifactHub struct{}

func (ah ArtifactHub) HandleDependents(comp Component, kc *kubernetes.Client, isDeploy bool) (summary string, err error) {
	source_uri := comp.Annotations[fmt.Sprintf("%s.model.source_uri", MesheryAnnotationPrefix)]
	act := kubernetes.UNINSTALL
	if isDeploy {
		act = kubernetes.INSTALL
	}

	if source_uri != "" {
		err = kc.ApplyHelmChart(kubernetes.ApplyHelmChartConfig{
			URL:                    source_uri,
			Namespace:              comp.Namespace,
			CreateNamespace:        true,
			Action:                 act,
			SkipUpgradeIfInstalled: true,
		})
		if err != nil {
			if !isDeploy {
				summary = fmt.Sprintf("error undeploying dependent helm chart for %s, please proceed with manual uninstall or try again", comp.Name)
			} else {
				summary = fmt.Sprintf("error deploying dependent helm chart for %s, please procced with manual install or try again", comp.Name)
			}
		} else {
			if !isDeploy {
				summary = fmt.Sprintf("Undeployed dependent helm chart for %s", comp.Name)
			} else {
				summary = fmt.Sprintf("Deployed dependent helm chart for %s", comp.Name)
			}
		}
	}
	return summary, err
}

func (ah ArtifactHub) String() string {
	return "artifacthub"
}

type Kubernetes struct{}

func (k Kubernetes) HandleDependents(comp Component, kc *kubernetes.Client, isDeploy bool) (summary string, err error) {
	return summary, err
}

func (k Kubernetes) String() string {
	return "kubernetes"
}
