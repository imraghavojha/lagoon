package presets

import "slices"

// Preset is a curated starting point for normal dev work and tiny AI runtimes.
type Preset struct {
	ID          string
	Name        string
	Description string
	Packages    []string
	Profile     string
	Up          map[string]string
	MinRAMMiB   int64
}

const NoneID = "custom"

var All = []Preset{
	{ID: NoneID, Name: "Custom", Description: "Start empty and choose packages yourself.", Profile: "minimal"},
	{ID: "python", Name: "Python", Description: "Python 3.11 with uv for fast project workflows.", Packages: []string{"python311", "uv"}, Profile: "network", MinRAMMiB: 1024},
	{ID: "node", Name: "Node", Description: "Node.js plus pnpm for frontend or API work.", Packages: []string{"nodejs_22", "pnpm"}, Profile: "network", MinRAMMiB: 1024},
	{ID: "go", Name: "Go", Description: "Go toolchain for compact static services.", Packages: []string{"go"}, Profile: "network", MinRAMMiB: 1024},
	{ID: "llama", Name: "llama.cpp", Description: "Tiny local model tools; bring your own model files.", Packages: []string{"llama-cpp"}, Profile: "minimal", MinRAMMiB: 4096},
	{ID: "whisper", Name: "whisper.cpp", Description: "Offline speech tooling with ffmpeg.", Packages: []string{"whisper-cpp", "ffmpeg"}, Profile: "minimal", MinRAMMiB: 2048},
}

func Find(id string) (Preset, bool) {
	for _, p := range All {
		if p.ID == id {
			return clone(p), true
		}
	}
	return Preset{}, false
}

func SafeForRAM(p Preset, ramMiB int64) bool {
	return ramMiB == 0 || p.MinRAMMiB == 0 || ramMiB >= p.MinRAMMiB
}

func RecommendedIDs(ramMiB int64) []string {
	ids := make([]string, 0, len(All))
	for _, p := range All {
		if SafeForRAM(p, ramMiB) {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func MergePackages(base, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	for _, p := range append(slices.Clone(base), extra...) {
		if p == "" || slices.Contains(out, p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func clone(p Preset) Preset {
	p.Packages = slices.Clone(p.Packages)
	if p.Up != nil {
		up := make(map[string]string, len(p.Up))
		for k, v := range p.Up {
			up[k] = v
		}
		p.Up = up
	}
	return p
}
