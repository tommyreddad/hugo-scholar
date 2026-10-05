package scholar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type SiteDefaults struct {
	Bibliography     string
	Style            string
	RemoveDuplicates bool
	Query            string
}

// ReadSiteDefaults asks Hugo to resolve its configuration, including config
// directories and environment overrides. Standalone BibTeX directories do not
// need Hugo.
func ReadSiteDefaults() (SiteDefaults, error) {
	defaults := SiteDefaults{Bibliography: "references", Style: "basic"}
	configured := false
	for _, filename := range []string{"hugo.toml", "hugo.yaml", "hugo.yml", "hugo.json", "config.toml", "config.yaml", "config.yml", "config.json", "config"} {
		if _, err := os.Stat(filename); err == nil {
			configured = true
			break
		} else if !os.IsNotExist(err) {
			return defaults, err
		}
	}
	if !configured {
		return defaults, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "hugo", "config", "--format", "json")
	var stderr strings.Builder
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return defaults, fmt.Errorf("read Hugo configuration: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var config struct {
		Params struct {
			Scholar struct {
				Bibliography     string
				Style            string
				RemoveDuplicates bool `json:"remove_duplicates"`
				Query            string
			}
		}
	}
	if err := json.Unmarshal(output, &config); err != nil {
		return defaults, fmt.Errorf("decode Hugo configuration: %w", err)
	}
	defaults.Bibliography = firstNonempty(config.Params.Scholar.Bibliography, defaults.Bibliography)
	defaults.Style = firstNonempty(config.Params.Scholar.Style, defaults.Style)
	defaults.RemoveDuplicates = config.Params.Scholar.RemoveDuplicates
	defaults.Query = config.Params.Scholar.Query
	return defaults, nil
}
