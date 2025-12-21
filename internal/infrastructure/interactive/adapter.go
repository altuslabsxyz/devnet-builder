// Package interactive provides infrastructure adapter for interactive prompts.
package interactive

import (
	"fmt"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/manifoldco/promptui"
)

// Adapter implements ports.InteractiveSelector using promptui.
type Adapter struct{}

// NewAdapter creates a new interactive adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

// NetworkOption represents a network option for selection.
type networkOption struct {
	Name        string
	Description string
}

// Default networks available for selection.
var networks = []networkOption{
	{Name: "mainnet", Description: "Mainnet network"},
	{Name: "testnet", Description: "Testnet network"},
}

// SelectNetwork prompts user to select a network.
func (a *Adapter) SelectNetwork() (string, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Name | cyan }} - {{ .Description | faint }}",
		Inactive: "  {{ .Name }} - {{ .Description | faint }}",
		Selected: "✓ {{ .Name | green }} selected",
	}

	prompt := promptui.Select{
		Label:     "Select network",
		Items:     networks,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()
	if err != nil {
		return "", adapterHandleInterruptError(err)
	}

	return networks[index].Name, nil
}

// versionItem represents a version for display in promptui.
type versionItem struct {
	TagName      string
	PublishedAt  string
	IsPrerelease bool
	IsLatest     bool
	IsCustom     bool
}

const adapterCustomRefOption = "[Custom branch/commit]"

// SelectVersion prompts user to select a version from releases.
func (a *Adapter) SelectVersion(prompt string, releases []ports.GitHubRelease, defaultVersion string) (string, bool, error) {
	if len(releases) == 0 {
		return "", false, fmt.Errorf("no versions available")
	}

	// Convert releases to versionItems, adding custom option at the end
	items := make([]versionItem, len(releases)+1)
	defaultIndex := 0
	for i, r := range releases {
		items[i] = versionItem{
			TagName:      r.TagName,
			PublishedAt:  r.PublishedAt.Format("2006-01-02"),
			IsPrerelease: r.Prerelease,
			IsLatest:     i == 0,
		}
		if r.TagName == defaultVersion {
			defaultIndex = i
		}
	}
	items[len(releases)] = versionItem{
		TagName:  adapterCustomRefOption,
		IsCustom: true,
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "{{ if .IsCustom }}▸ {{ .TagName | magenta }}{{ else }}▸ {{ .TagName | cyan }} - {{ .PublishedAt }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ else if .IsPrerelease }} {{ \"(pre-release)\" | yellow }}{{ end }}{{ end }}",
		Inactive: "{{ if .IsCustom }}  {{ .TagName | faint }}{{ else }}  {{ .TagName }} - {{ .PublishedAt }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ else if .IsPrerelease }} {{ \"(pre-release)\" | yellow }}{{ end }}{{ end }}",
		Selected: "{{ if .IsCustom }}✓ Custom ref{{ else }}✓ {{ .TagName | green }}{{ end }}",
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		if item.IsCustom {
			return true
		}
		return strings.Contains(strings.ToLower(item.TagName), strings.ToLower(strings.TrimSpace(input)))
	}

	selectPrompt := promptui.Select{
		Label:     prompt,
		Items:     items,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
		CursorPos: defaultIndex,
	}

	index, _, err := selectPrompt.Run()
	if err != nil {
		return "", false, adapterHandleInterruptError(err)
	}

	selected := items[index]
	if selected.IsCustom {
		ref, err := a.promptCustomRef()
		return ref, true, err
	}

	return selected.TagName, false, nil
}

// promptCustomRef prompts for a custom branch or commit hash.
func (a *Adapter) promptCustomRef() (string, error) {
	validate := func(input string) error {
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			return fmt.Errorf("reference cannot be empty")
		}
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
	}

	result, err := prompt.Run()
	if err != nil {
		return "", adapterHandleInterruptError(err)
	}

	return strings.TrimSpace(result), nil
}

// dockerImageItem represents a docker image for display.
type dockerImageItem struct {
	Tag       string
	CreatedAt string
	IsLatest  bool
	IsCustom  bool
}

const adapterCustomImageOption = "[Enter custom image...]"

// SelectDockerImage prompts user to select a docker image version.
func (a *Adapter) SelectDockerImage(prompt string, versions []ports.ImageVersion, defaultVersion string) (string, bool, error) {
	if len(versions) == 0 {
		return "", false, fmt.Errorf("no docker image versions available")
	}

	items := make([]dockerImageItem, len(versions)+1)
	defaultIndex := 0
	for i, v := range versions {
		items[i] = dockerImageItem{
			Tag:       v.Tag,
			CreatedAt: v.CreatedAt.Format("2006-01-02"),
			IsLatest:  v.IsLatest,
		}
		if v.Tag == defaultVersion {
			defaultIndex = i
		}
	}
	items[len(versions)] = dockerImageItem{
		Tag:      adapterCustomImageOption,
		IsCustom: true,
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "{{ if .IsCustom }}▸ {{ .Tag | magenta }}{{ else }}▸ {{ .Tag | cyan }} - {{ .CreatedAt }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ end }}{{ end }}",
		Inactive: "{{ if .IsCustom }}  {{ .Tag | faint }}{{ else }}  {{ .Tag }} - {{ .CreatedAt }}{{ if .IsLatest }} {{ \"(latest)\" | green }}{{ end }}{{ end }}",
		Selected: "{{ if .IsCustom }}✓ Custom image{{ else }}✓ {{ .Tag | green }}{{ end }}",
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		if item.IsCustom {
			return true
		}
		return strings.Contains(strings.ToLower(item.Tag), strings.ToLower(strings.TrimSpace(input)))
	}

	selectPrompt := promptui.Select{
		Label:     prompt,
		Items:     items,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
		CursorPos: defaultIndex,
	}

	index, _, err := selectPrompt.Run()
	if err != nil {
		return "", false, adapterHandleInterruptError(err)
	}

	selected := items[index]
	if selected.IsCustom {
		imageURL, err := a.promptCustomImage()
		return imageURL, true, err
	}

	return selected.Tag, false, nil
}

// promptCustomImage prompts for a custom docker image URL.
func (a *Adapter) promptCustomImage() (string, error) {
	validate := func(input string) error {
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			return fmt.Errorf("image URL cannot be empty")
		}
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
	}

	result, err := prompt.Run()
	if err != nil {
		return "", adapterHandleInterruptError(err)
	}

	return strings.TrimSpace(result), nil
}

// PromptUpgradeName prompts user for an upgrade name.
func (a *Adapter) PromptUpgradeName(defaultName string) (string, error) {
	suggestedName := defaultName
	if suggestedName == "" {
		suggestedName = "upgrade"
	}
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
	}

	result, err := prompt.Run()
	if err != nil {
		return "", adapterHandleInterruptError(err)
	}

	return strings.TrimSpace(result), nil
}

// ConfirmSelection asks user to confirm their selection.
func (a *Adapter) ConfirmSelection(config *ports.SelectionConfig) (bool, error) {
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
		return false, adapterHandleInterruptError(err)
	}

	return true, nil
}

// ConfirmUpgradeSelection asks user to confirm upgrade selection.
func (a *Adapter) ConfirmUpgradeSelection(config *ports.UpgradeSelectionConfig) (bool, error) {
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
		return false, adapterHandleInterruptError(err)
	}

	return true, nil
}

// ConfirmAction asks user to confirm a generic action.
func (a *Adapter) ConfirmAction(message string) (bool, error) {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
		Default:   "y",
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, adapterHandleInterruptError(err)
	}

	return true, nil
}

// adapterHandleInterruptError converts promptui errors to ports.CancellationError.
func adapterHandleInterruptError(err error) error {
	if err == promptui.ErrInterrupt {
		return &ports.CancellationError{Message: "Operation cancelled"}
	}
	if err == promptui.ErrEOF {
		return &ports.CancellationError{Message: "Operation cancelled (EOF)"}
	}
	return err
}
