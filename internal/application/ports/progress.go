package ports

// StepProgress represents progress for a long-running operation.
type StepProgress struct {
	Name    string // "Downloading snapshot", "Extracting...", etc.
	Status  string // "running", "completed", "failed"
	Current int64  // bytes downloaded, files processed, etc. (0 if indeterminate)
	Total   int64  // total bytes, total files, etc. (0 if unknown)
	Unit    string // "bytes", "files", "" for indeterminate
	Detail  string // "from cache", "v1.2.0", etc.
	Error   string // error message if failed
}

// ProgressReporter allows operations to report progress updates.
type ProgressReporter interface {
	ReportStep(step StepProgress)
}

// ProgressFunc is a simple function adapter for ProgressReporter.
type ProgressFunc func(step StepProgress)

// ReportStep implements ProgressReporter.
func (f ProgressFunc) ReportStep(step StepProgress) {
	if f != nil {
		f(step)
	}
}

// NilProgressReporter is a no-op progress reporter for when progress isn't needed.
var NilProgressReporter ProgressReporter = ProgressFunc(nil)
