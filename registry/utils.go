package registry

import "fmt"

func FormatSheetURL(baseURL string, sheetID int64) string {
	return fmt.Sprintf("%s/export?format=csv&gid=%d", baseURL, sheetID)
}
