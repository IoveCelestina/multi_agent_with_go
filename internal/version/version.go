package version

const (
	Name    = "agentctl"
	Version = "0.1.0-dev"
)

func String() string {
	return Name + " " + Version
}
