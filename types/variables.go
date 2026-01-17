package types

var (
	HomeDir string = ""
)

func SetHomeDir(dir string) {
	HomeDir = dir
}

func GetHomeDir() string {
	return HomeDir
}
