package interactive

import (
	"fmt"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/github"
	"github.com/manifoldco/promptui"
)

// SelectNetwork prompts the user to select a network.
func SelectNetwork() (string, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Name | cyan }} - {{ .Description | faint }}",
		Inactive: "  {{ .Name }} - {{ .Description | faint }}",
		Selected: "✓ {{ .Name | green }} selected",
	}

	prompt := promptui.Select{
		Label:     "Select network",
		Items:     Networks,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return Networks[index].Name, nil
}

// customRefOption is a special marker for custom branch/commit input.
const customRefOption = "[Custom branch/commit]"

// SelectVersion prompts the user to select a version from the list.
// Users can also enter a custom branch name or commit hash.
// Returns the selected version and whether it's a custom ref.
func SelectVersion(label string, releases []github.GitHubRelease, defaultVersion string) (string, bool, error) {
	if len(releases) == 0 {
		return "", false, fmt.Errorf("no versions available")
	}

	// Convert releases to VersionItems, adding custom option at the end
	items := make([]VersionItem, len(releases)+1)
	defaultIndex := 0
	for i, r := range releases {
		items[i] = VersionItem{
			TagName:      r.TagName,
			PublishedAt:  r.PublishedAt,
			IsPrerelease: r.Prerelease,
			IsLatest:     i == 0, // First release is latest
		}
		if r.TagName == defaultVersion {
			defaultIndex = i
		}
	}
	// Add custom ref option at the end
	items[len(releases)] = VersionItem{
		TagName:  customRefOption,
		IsCustom: true,
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "{{ if .IsCustom }}▸ {{ .TagName | magenta }}{{ else }}▸ {{ .TagName | cyan }} - {{ .PublishedAt.Format \"2006-01-02\" }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ else if .IsPrerelease }} {{ \"(pre-release)\" | yellow }}{{ end }}{{ end }}",
		Inactive: "{{ if .IsCustom }}  {{ .TagName | faint }}{{ else }}  {{ .TagName }} - {{ .PublishedAt.Format \"2006-01-02\" }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ else if .IsPrerelease }} {{ \"(pre-release)\" | yellow }}{{ end }}{{ end }}",
		Selected: "{{ if .IsCustom }}✓ Custom ref{{ else }}✓ {{ .TagName | green }}{{ end }}",
		Details: `{{ if not .IsCustom }}
--------- Version Details ----------
{{ "Tag:" | faint }}      {{ .TagName }}
{{ "Released:" | faint }} {{ .PublishedAt.Format "2006-01-02 15:04:05" }}
{{ if .IsPrerelease }}{{ "Note:" | faint }}     Pre-release version{{ end }}{{ else }}
--------- Custom Reference ----------
{{ "Enter:" | faint }}    Branch name or commit hash
{{ "Note:" | faint }}     Will build from source using goreleaser{{ end }}`,
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		// Always show custom option when searching
		if item.IsCustom {
			return true
		}
		tagName := strings.ToLower(item.TagName)
		input = strings.ToLower(strings.TrimSpace(input))
		return strings.Contains(tagName, input)
	}

	prompt := promptui.Select{
		Label:             label,
		Items:             items,
		Templates:         templates,
		Size:              10,
		Searcher:          searcher,
		StartInSearchMode: false,
		CursorPos:         defaultIndex,
	}

	index, _, err := prompt.Run()
	if err != nil {
		return "", false, err
	}

	selected := items[index]

	// If custom ref selected, prompt for the ref
	if selected.IsCustom {
		ref, err := promptCustomRef()
		return ref, true, err
	}

	return selected.TagName, false, nil
}

// promptCustomRef prompts the user to enter a custom branch or commit hash.
func promptCustomRef() (string, error) {
	validate := func(input string) error {
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			return fmt.Errorf("reference cannot be empty")
		}
		// Basic validation: no spaces, reasonable length
		if strings.Contains(input, " ") {
			return fmt.Errorf("reference cannot contain spaces")
		}
		if len(input) > 100 {
			return fmt.Errorf("reference too long")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Enter branch name or commit hash",
		Validate: validate,
		Templates: &promptui.PromptTemplates{
			Prompt:  "{{ . }}: ",
			Valid:   "{{ . | green }}: ",
			Invalid: "{{ . | red }}: ",
			Success: "✓ Custom ref: ",
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result), nil
}

// ConfirmSelection prompts the user to confirm their selection for start command.
func ConfirmSelection(config *SelectionConfig) (bool, error) {
	fmt.Printf("\nStarting devnet with:\n")
	fmt.Printf("  Network: %s\n", config.Network)

	exportSuffix := ""
	if config.ExportIsCustomRef {
		exportSuffix = " (custom ref - will build from source)"
	}
	fmt.Printf("  Export version: %s%s\n", config.ExportVersion, exportSuffix)

	startSuffix := ""
	if config.StartIsCustomRef {
		startSuffix = " (custom ref - will build from source)"
	}
	fmt.Printf("  Start version: %s%s\n", config.StartVersion, startSuffix)
	fmt.Println()

	prompt := promptui.Prompt{
		Label:     "Proceed",
		IsConfirm: true,
		Default:   "y",
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// PromptUpgradeName prompts the user to enter an upgrade handler name.
func PromptUpgradeName(defaultName string) (string, error) {
	// Generate a suggested name from version
	suggestedName := defaultName
	if suggestedName == "" {
		suggestedName = "upgrade"
	}
	// Remove 'v' prefix if present for cleaner name
	if len(suggestedName) > 0 && suggestedName[0] == 'v' {
		suggestedName = suggestedName[1:] + "-upgrade"
	} else {
		suggestedName += "-upgrade"
	}

	validate := func(input string) error {
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			return fmt.Errorf("upgrade name cannot be empty")
		}
		if strings.Contains(input, " ") {
			return fmt.Errorf("upgrade name cannot contain spaces")
		}
		if len(input) > 50 {
			return fmt.Errorf("upgrade name too long (max 50 characters)")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Enter upgrade handler name",
		Default:  suggestedName,
		Validate: validate,
		Templates: &promptui.PromptTemplates{
			Prompt:  "{{ . }}: ",
			Valid:   "{{ . | green }}: ",
			Invalid: "{{ . | red }}: ",
			Success: "✓ Upgrade name: ",
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result), nil
}

// ConfirmUpgradeSelection prompts the user to confirm their upgrade selection.
func ConfirmUpgradeSelection(config *UpgradeSelectionConfig) (bool, error) {
	fmt.Printf("\nUpgrade configuration:\n")
	fmt.Printf("  Upgrade name: %s\n", config.UpgradeName)

	versionSuffix := ""
	if config.IsCustomRef {
		versionSuffix = " (custom ref - will build from source)"
	}
	fmt.Printf("  Target version: %s%s\n", config.UpgradeVersion, versionSuffix)
	fmt.Println()

	prompt := promptui.Prompt{
		Label:     "Proceed with upgrade",
		IsConfirm: true,
		Default:   "y",
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// customImageOption is a special marker for custom docker image input.
const customImageOption = "[Enter custom image...]"

// SelectDockerImage prompts the user to select a docker image version from GHCR.
// Users can also enter a custom image URL.
// Returns the selected image tag (or full URL for custom), and whether it's a custom image.
func SelectDockerImage(versions []github.ImageVersion) (string, bool, error) {
	if len(versions) == 0 {
		return "", false, fmt.Errorf("no docker image versions available")
	}

	// Convert to DockerImageItems, adding custom option at the end
	items := make([]DockerImageItem, len(versions)+1)
	for i, v := range versions {
		items[i] = DockerImageItem{
			Tag:       v.Tag,
			CreatedAt: v.CreatedAt,
			IsLatest:  v.IsLatest,
		}
	}
	// Add custom image option at the end
	items[len(versions)] = DockerImageItem{
		Tag:      customImageOption,
		IsCustom: true,
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "{{ if .IsCustom }}▸ {{ .Tag | magenta }}{{ else }}▸ {{ .Tag | cyan }} - {{ .CreatedAt.Format \"2006-01-02\" }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ end }}{{ end }}",
		Inactive: "{{ if .IsCustom }}  {{ .Tag | faint }}{{ else }}  {{ .Tag }} - {{ .CreatedAt.Format \"2006-01-02\" }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ end }}{{ end }}",
		Selected: "{{ if .IsCustom }}✓ Custom image{{ else }}✓ {{ .Tag | green }}{{ end }}",
		Details: `{{ if not .IsCustom }}
--------- Image Details ----------
{{ "Tag:" | faint }}      {{ .Tag }}
{{ "Created:" | faint }}  {{ .CreatedAt.Format "2006-01-02 15:04:05" }}{{ else }}
--------- Custom Image ----------
{{ "Enter:" | faint }}    Full image URL (e.g., myregistry.io/image:tag){{ end }}`,
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		// Always show custom option when searching
		if item.IsCustom {
			return true
		}
		tag := strings.ToLower(item.Tag)
		input = strings.ToLower(strings.TrimSpace(input))
		return strings.Contains(tag, input)
	}

	prompt := promptui.Select{
		Label:             "Select docker image version",
		Items:             items,
		Templates:         templates,
		Size:              10,
		Searcher:          searcher,
		StartInSearchMode: false,
	}

	index, _, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			return "", false, &CancellationError{Message: "operation cancelled by user"}
		}
		return "", false, err
	}

	selected := items[index]

	// If custom image selected, prompt for the image URL
	if selected.IsCustom {
		imageURL, err := promptCustomImage()
		if err != nil {
			if err == promptui.ErrInterrupt {
				return "", false, &CancellationError{Message: "operation cancelled by user"}
			}
			return "", false, err
		}
		return imageURL, true, nil
	}

	return selected.Tag, false, nil
}

// promptCustomImage prompts the user to enter a custom docker image URL.
func promptCustomImage() (string, error) {
	validate := func(input string) error {
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			return fmt.Errorf("image URL cannot be empty")
		}
		// Basic validation: no spaces
		if strings.Contains(input, " ") {
			return fmt.Errorf("image URL cannot contain spaces")
		}
		if len(input) > 200 {
			return fmt.Errorf("image URL too long")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Enter docker image URL",
		Validate: validate,
		Templates: &promptui.PromptTemplates{
			Prompt:  "{{ . }}: ",
			Valid:   "{{ . | green }}: ",
			Invalid: "{{ . | red }}: ",
			Success: "✓ Docker image: ",
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result), nil
}

// ConfirmReplaceSelection prompts the user to confirm their replace selection.
func ConfirmReplaceSelection(version string) (bool, error) {
	prompt := promptui.Prompt{
		Label:     "Proceed with binary replacement",
		IsConfirm: true,
		Default:   "y",
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
