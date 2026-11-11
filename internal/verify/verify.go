package verify

import (
	"errors"
	"os/exec"

	"hdf/internal/helium"
)

type Stream struct {
	Name   string
	URL    string
	Prompt string
}

var Streams = []Stream{
	{
		Name:   "DRM Sense",
		URL:    "https://drmsense.netlify.app/",
		Prompt: "does it say Widevine is supported? y / n",
	},
	{
		Name:   "Bitmovin DRM demo",
		URL:    "https://bitmovin.com/demos/drm",
		Prompt: "did the video play? y / n",
	},
}

var ErrNoBinary = errors.New("Helium binary not found")

func Default() Stream { return Streams[0] }

func Open(install helium.Install, stream Stream) error {
	if install.Exe == "" {
		return ErrNoBinary
	}

	command := exec.Command(install.Exe, stream.URL)
	if err := command.Start(); err != nil {
		return err
	}

	return command.Process.Release()
}
