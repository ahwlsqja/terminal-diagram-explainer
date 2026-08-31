package sequence

type Kind uint8

const (
	Request Kind = iota
	Return
)

type StepKind uint8

const (
	MessageStep StepKind = iota
	FragmentStartStep
	FragmentBranchStep
	FragmentEndStep
	ActivateStep
	DeactivateStep
)

type FragmentKind uint8

const (
	LoopFragment FragmentKind = iota
	AltFragment
	OptFragment
)

type Participant struct {
	ID    string
	Label string
}

type Message struct {
	From  int
	To    int
	Label string
	Kind  Kind
}

type Step struct {
	Kind        StepKind
	Message     Message
	Participant int
	Fragment    FragmentKind
	Label       string
}

type Diagram struct {
	Participants []Participant
	Messages     []Message
	Steps        []Step
}

type Limits struct {
	MaxSourceBytes     int
	MaxLines           int
	MaxParticipants    int
	MaxMessages        int
	MaxSteps           int
	MaxFragments       int
	MaxFragmentDepth   int
	MaxActivations     int
	MaxActivationDepth int
	MaxIDBytes         int
	MaxLabelCells      int
}

func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes:     256 * 1024,
		MaxLines:           2048,
		MaxParticipants:    16,
		MaxMessages:        96,
		MaxSteps:           256,
		MaxFragments:       32,
		MaxFragmentDepth:   8,
		MaxActivations:     96,
		MaxActivationDepth: 8,
		MaxIDBytes:         64,
		MaxLabelCells:      96,
	}
}
