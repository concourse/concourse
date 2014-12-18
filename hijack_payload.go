package atc

type HijackInput struct {
	Stdin   []byte         `json:"stdin,omitempty"`
	TTYSpec *HijackTTYSpec `json:"tty,omitempty"`
}

type HijackTTYSpec struct {
	WindowSize HijackWindowSize `json:"window_size"`
}

type HijackWindowSize struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

type HijackOutput struct {
	Stdout     []byte `json:"stdout,omitempty"`
	Stderr     []byte `json:"stderr,omitempty"`
	Error      string `json:"error,omitempty"`
	ExitStatus *int   `json:"exit_status,omitempty"`
}
