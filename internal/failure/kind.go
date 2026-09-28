package failure

type Kind int

const (
	Unknown Kind = -1
	Own     Kind = 0
	Start   Kind = 1
	Exit    Kind = 2
)

func (kind Kind) String() string {
	switch kind {
	case Own:
		return "Own"
	case Start:
		return "Start"
	case Exit:
		return "Exit"
	default:
		return "Unknown"
	}
}

func Parse(value string) Kind {
	switch value {
	case "Own":
		return Own
	case "Start":
		return Start
	case "Exit":
		return Exit
	default:
		return Unknown
	}
}
